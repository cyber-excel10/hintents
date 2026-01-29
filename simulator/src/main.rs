// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

mod cli;
mod config;
mod gas_optimizer;
mod ipc;
mod theme;

use base64::Engine as _;
use serde::{Deserialize, Serialize};
use serde_json::json;
use soroban_env_host::events::Events;
use soroban_env_host::xdr::ReadXdr;
use std::collections::HashMap;
use std::io::{self, Read};
use std::panic;

use gas_optimizer::{BudgetMetrics, GasOptimizationAdvisor, OptimizationReport};

#[derive(Debug, Deserialize)]
struct SimulationRequest {
    envelope_xdr: String,
    result_meta_xdr: String,
    ledger_entries: Option<HashMap<String, String>>,
    timestamp: Option<i64>,
    ledger_sequence: Option<u32>,
    // Optional: Path to local WASM file for local replay
    wasm_path: Option<String>,
    // Optional: Mock arguments for local replay (JSON array of strings)
    mock_args: Option<Vec<String>>,
    profile: Option<bool>,
    #[serde(default)]
    enable_optimization_advisor: bool,
}

#[derive(Debug, Serialize, Clone)]
struct CategorizedEvent {
    event_type: String,
    contract_id: Option<String>,
    topics: Vec<String>,
    data: String,
}

#[derive(Debug, Serialize)]
struct DiagnosticEvent {
    event_type: String,
    contract_id: Option<String>,
    topics: Vec<String>,
    data: String,
    in_successful_contract_call: bool,
}

#[derive(Debug, Serialize)]
struct SimulationResponse {
    status: String,
    error: Option<String>,
    events: Vec<String>,
    diagnostic_events: Vec<DiagnosticEvent>,
    categorized_events: Vec<CategorizedEvent>,
    logs: Vec<String>,
    flamegraph: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    optimization_report: Option<OptimizationReport>,
    #[serde(skip_serializing_if = "Option::is_none")]
    budget_usage: Option<BudgetUsage>,
}

#[derive(Debug, Serialize)]
struct BudgetUsage {
    cpu_instructions: u64,
    memory_bytes: u64,
    operations_count: usize,
}

fn categorize_event_for_analyzer(
    event: &soroban_env_host::events::HostEvent,
) -> Result<String, String> {
    use soroban_env_host::xdr::{ContractEventBody, ContractEventType, ScVal};

    let contract_id = match &event.event.contract_id {
        Some(id) => format!("{:?}", id),
        None => "unknown".to_string(),
    };

    let event_type_str = match &event.event.type_ {
        ContractEventType::Contract => "contract",
        ContractEventType::System => "system",
        ContractEventType::Diagnostic => "diagnostic",
    };

    let (topics, _data_val) = match &event.event.body {
        ContractEventBody::V0(v0) => (&v0.topics, &v0.data),
    };

    let event_json = if let Some(first_topic) = topics.get(0) {
        let topic_str = format!("{:?}", first_topic);

        if topic_str.contains("require_auth") {
            let address = if let ScVal::Address(addr) = first_topic {
                format!("{:?}", addr)
            } else {
                "unknown".to_string()
            };

            json!({
                "type": "auth",
                "contract": contract_id,
                "address": address,
                "event_type": event_type_str,
            })
            .to_string()
        } else if topic_str.contains("set")
            || topic_str.contains("write")
            || topic_str.contains("storage")
        {
            json!({
                "type": "storage_write",
                "contract": contract_id,
                "event_type": event_type_str,
            })
            .to_string()
        } else if topic_str.contains("call") || topic_str.contains("invoke") {
            if let ScVal::Symbol(sym) = first_topic {
                json!({
                    "type": "contract_call",
                    "contract": contract_id,
                    "function": sym.to_string(),
                    "event_type": event_type_str,
                })
                .to_string()
            } else {
                json!({
                    "type": "contract_call",
                    "contract": contract_id,
                    "event_type": event_type_str,
                })
                .to_string()
            }
        } else {
            json!({
                "type": "other",
                "contract": contract_id,
                "event_type": event_type_str,
            })
            .to_string()
        }
    } else {
        json!({
            "type": "other",
            "contract": contract_id,
            "event_type": event_type_str,
        })
        .to_string()
    };

    Ok(event_json)
}

#[derive(Debug, Serialize, Deserialize)]
struct StructuredError {
    error_type: String,
    message: String,
    details: Option<String>,
}

fn main() {
    // Read JSON from Stdin
    let mut buffer = String::new();
    if let Err(e) = io::stdin().read_to_string(&mut buffer) {
        let res = SimulationResponse {
            status: "error".to_string(),
            error: Some(format!("Failed to read stdin: {}", e)),
            events: vec![],
            diagnostic_events: vec![],
            categorized_events: vec![],
            logs: vec![],
            flamegraph: None,
            optimization_report: None,
            budget_usage: None,
        };
        println!("{}", serde_json::to_string(&res).unwrap());
        return;
    }

    // Parse Request
    let request: SimulationRequest = match serde_json::from_str(&buffer) {
        Ok(req) => req,
        Err(e) => {
            let res = SimulationResponse {
                status: "error".to_string(),
                error: Some(format!("Invalid JSON: {}", e)),
                events: vec![],
                diagnostic_events: vec![],
                categorized_events: vec![],
                logs: vec![],
                flamegraph: None,
                optimization_report: None,
                budget_usage: None,
            };
            println!("{}", serde_json::to_string(&res).unwrap());
            return;
        }
    };

    // Check if this is a local WASM replay (no network data)
    if let Some(wasm_path) = &request.wasm_path {
        return run_local_wasm_replay(wasm_path, &request.mock_args);
    }

    // Decode Envelope XDR
    let envelope = match base64::engine::general_purpose::STANDARD.decode(&request.envelope_xdr) {
        Ok(bytes) => match soroban_env_host::xdr::TransactionEnvelope::from_xdr(
            &bytes,
            soroban_env_host::xdr::Limits::none(),
        ) {
            Ok(env) => env,
            Err(e) => {
                return send_error(format!("Failed to parse Envelope XDR: {}", e));
            }
        },
        Err(e) => {
            return send_error(format!("Failed to decode Envelope Base64: {}", e));
        }
    };

    // Initialize Host
    let host = soroban_env_host::Host::default();
    host.set_diagnostic_level(soroban_env_host::DiagnosticLevel::Debug)
        .unwrap();

    // Override Ledger Info if provided
    if request.timestamp.is_some() || request.ledger_sequence.is_some() {
        host.with_mut_ledger_info(|ledger_info| {
            if let Some(ts) = request.timestamp {
                ledger_info.timestamp = ts as u64;
            }
            if let Some(seq) = request.ledger_sequence {
                ledger_info.sequence_number = seq;
            }
        })
        .unwrap();
    }
    // Populate Host Storage
    let mut loaded_entries_count = 0;
    if let Some(entries) = &request.ledger_entries {
        for (key_xdr, entry_xdr) in entries {
            // Decode Key
            let _key = match base64::engine::general_purpose::STANDARD.decode(key_xdr) {
                Ok(b) => match soroban_env_host::xdr::LedgerKey::from_xdr(
                    &b,
                    soroban_env_host::xdr::Limits::none(),
                ) {
                    Ok(k) => k,
                    Err(e) => return send_error(format!("Failed to parse LedgerKey XDR: {}", e)),
                },
                Err(e) => return send_error(format!("Failed to decode LedgerKey Base64: {}", e)),
            };

            // Decode Entry
            let _entry = match base64::engine::general_purpose::STANDARD.decode(entry_xdr) {
                Ok(b) => match soroban_env_host::xdr::LedgerEntry::from_xdr(
                    &b,
                    soroban_env_host::xdr::Limits::none(),
                ) {
                    Ok(e) => e,
                    Err(e) => return send_error(format!("Failed to parse LedgerEntry XDR: {}", e)),
                },
                Err(e) => return send_error(format!("Failed to decode LedgerEntry Base64: {}", e)),
            };

            // In real implementation, we'd inject into host storage here.
            loaded_entries_count += 1;
        }
    }

    // Extract Operations from Envelope
    let operations = match &envelope {
        soroban_env_host::xdr::TransactionEnvelope::Tx(tx_v1) => &tx_v1.tx.operations,
        soroban_env_host::xdr::TransactionEnvelope::TxV0(tx_v0) => &tx_v0.tx.operations,
        soroban_env_host::xdr::TransactionEnvelope::TxFeeBump(bump) => match &bump.tx.inner_tx {
            soroban_env_host::xdr::FeeBumpTransactionInnerTx::Tx(tx_v1) => &tx_v1.tx.operations,
        },
    };

    // Wrap the operation execution in panic protection
    let result = panic::catch_unwind(panic::AssertUnwindSafe(|| {
        execute_operations(&host, operations)
    }));

    // Budget and Reporting
    let budget = host.budget_cloned();
    let cpu_insns = budget.get_cpu_insns_consumed().unwrap_or(0);
    let mem_bytes = budget.get_mem_bytes_consumed().unwrap_or(0);

    let budget_usage = BudgetUsage {
        cpu_instructions: cpu_insns,
        memory_bytes: mem_bytes,
        operations_count: operations.as_slice().len(),
    };

    let optimization_report = if request.enable_optimization_advisor {
        let advisor = GasOptimizationAdvisor::new();
        let metrics = BudgetMetrics {
            cpu_instructions: budget_usage.cpu_instructions,
            memory_bytes: budget_usage.memory_bytes,
            total_operations: budget_usage.operations_count,
        };
        Some(advisor.analyze(&metrics))
    } else {
        None
    };

    let mut flamegraph_svg = None;
    if request.profile.unwrap_or(false) {
        // Simple simulated flamegraph for demonstration
        let folded_data = format!("Total;CPU {}\nTotal;Memory {}\n", cpu_insns, mem_bytes);
        let mut result = Vec::new();
        let mut options = inferno::flamegraph::Options::default();
        options.title = "Soroban Resource Consumption".to_string();

        if let Err(e) =
            inferno::flamegraph::from_reader(&mut options, folded_data.as_bytes(), &mut result)
        {
            eprintln!("Failed to generate flamegraph: {}", e);
        } else {
            flamegraph_svg = Some(String::from_utf8_lossy(&result).to_string());
        }
    }

    match result {
        Ok(Ok(exec_logs)) => {
            // Extract both raw event strings and structured diagnostic events
            let (events, diagnostic_events) = match host.get_events() {
                Ok(evs) => {
                    let raw_events: Vec<String> =
                        evs.0.iter().map(|e| format!("{:?}", e)).collect();
                    let diag_events: Vec<DiagnosticEvent> = evs
                        .0
                        .iter()
                        .enumerate()
                        .map(|(_idx, event)| {
                            let event_type = match &event.event.type_ {
                                soroban_env_host::xdr::ContractEventType::Contract => {
                                    "contract".to_string()
                                }
                                soroban_env_host::xdr::ContractEventType::System => {
                                    "system".to_string()
                                }
                                soroban_env_host::xdr::ContractEventType::Diagnostic => {
                                    "diagnostic".to_string()
                                }
                            };

                            let contract_id = if let Some(contract_id) = &event.event.contract_id {
                                Some(format!("{:?}", contract_id))
                            } else {
                                None
                            };

                            let (topics, data) = match &event.event.body {
                                soroban_env_host::xdr::ContractEventBody::V0(v0) => {
                                    let topics: Vec<String> =
                                        v0.topics.iter().map(|t| format!("{:?}", t)).collect();
                                    let data = format!("{:?}", v0.data);
                                    (topics, data)
                                }
                            };

                            DiagnosticEvent {
                                event_type,
                                contract_id,
                                topics,
                                data,
                                in_successful_contract_call: event.failed_call,
                            }
                        })
                        .collect();
                    (raw_events, diag_events)
                }
                Err(_) => (vec!["Failed to retrieve events".to_string()], vec![]),
            };

            // Capture categorized events for analyzer
            let categorized_events = match host.get_events() {
                Ok(evs) => categorize_events(&evs),
                Err(_) => vec![],
            };

            let mut final_logs = vec![
                format!("Host Initialized with Budget: {:?}", budget),
                format!("Loaded {} Ledger Entries", loaded_entries_count),
                format!("Captured {} diagnostic events", diagnostic_events.len()),
                format!("CPU Instructions Used: {}", cpu_insns),
                format!("Memory Bytes Used: {}", mem_bytes),
            ];
            for log in exec_logs {
                final_logs.push(log);
            }

            let response = SimulationResponse {
                status: "success".to_string(),
                error: None,
                events,
                diagnostic_events,
                categorized_events,
                logs: final_logs,
                flamegraph: flamegraph_svg,
                optimization_report,
                budget_usage: Some(budget_usage),
            };

            println!("{}", serde_json::to_string(&response).unwrap());
        }
        Ok(Err(host_error)) => {
            // Host error during execution (e.g., contract trap, validation failure)
            let structured_error = StructuredError {
                error_type: "HostError".to_string(),
                message: format!("{:?}", host_error),
                details: Some(format!(
                    "Contract execution failed with host error: {:?}",
                    host_error
                )),
            };

            let response = SimulationResponse {
                status: "error".to_string(),
                error: Some(serde_json::to_string(&structured_error).unwrap()),
                events: vec![],
                diagnostic_events: vec![],
                categorized_events: vec![],
                logs: vec![],
                flamegraph: None,
                optimization_report: None,
                budget_usage: None,
            };

            println!("{}", serde_json::to_string(&response).unwrap());
        }
        Err(panic_info) => {
            let panic_msg = if let Some(s) = panic_info.downcast_ref::<&str>() {
                s.to_string()
            } else if let Some(s) = panic_info.downcast_ref::<String>() {
                s.clone()
            } else {
                "Unknown panic".to_string()
            };

            let response = SimulationResponse {
                status: "error".to_string(),
                error: Some(format!("Simulator panicked: {}", panic_msg)),
                events: vec![],
                diagnostic_events: vec![],
                categorized_events: vec![],
                logs: vec![format!("PANIC: {}", panic_msg)],
                flamegraph: None,
                optimization_report: None,
                budget_usage: None,
            };
            println!("{}", serde_json::to_string(&response).unwrap());
        }
    }
}

fn execute_operations(
    _host: &soroban_env_host::Host,
    operations: &soroban_env_host::xdr::VecM<soroban_env_host::xdr::Operation, 100>,
) -> Result<Vec<String>, soroban_env_host::HostError> {
    let mut logs = vec![];

    for op in operations.iter() {
        if let soroban_env_host::xdr::OperationBody::InvokeHostFunction(host_fn_op) = &op.body {
            match &host_fn_op.host_function {
                soroban_env_host::xdr::HostFunction::InvokeContract(invoke_args) => {
                    logs.push("Found InvokeContract operation!".to_string());

                    let address = &invoke_args.contract_address;
                    let func_name = &invoke_args.function_name;
                    let invoke_args_vec = &invoke_args.args;

                    logs.push(format!("About to Invoke Contract: {:?}", address));
                    logs.push(format!("Function: {:?}", func_name));
                    logs.push(format!("Args Count: {}", invoke_args_vec.len()));
                }
                _ => {
                    logs.push("Skipping non-InvokeContract Host Function".to_string());
                }
            }
        }
    }
    Ok(logs)
}

fn categorize_events(events: &Events) -> Vec<CategorizedEvent> {
    use soroban_env_host::xdr::{ContractEventBody, ContractEventType, ScVal};

    events
        .0
        .iter()
        .filter_map(|event| {
            // Access body to get topics and data
            let (topics, data_val) = match &event.event.body {
                ContractEventBody::V0(v0) => (&v0.topics, &v0.data),
            };

            if !event.failed_call {
                let event_type = match &event.event.type_ {
                    ContractEventType::Contract => {
                        if let Some(topic) = topics.get(0) {
                            if let ScVal::Symbol(sym) = topic {
                                match sym.to_string().as_str() {
                                    s if s.contains("require_auth") => "require_auth",
                                    s if s.contains("set") || s.contains("write") => {
                                        "storage_write"
                                    }
                                    _ => "contract",
                                }
                            } else {
                                "contract"
                            }
                        } else {
                            "contract"
                        }
                    }
                    ContractEventType::System => "system",
                    ContractEventType::Diagnostic => {
                        if let Some(topic) = topics.get(0) {
                            if let ScVal::Symbol(sym) = topic {
                                match sym.to_string().as_str() {
                                    s if s.contains("fn_call") => "invocation",
                                    s if s.contains("fn_return") => "return",
                                    _ => "diagnostic",
                                }
                            } else {
                                "diagnostic"
                            }
                        } else {
                            "diagnostic"
                        }
                    }
                };

                Some(CategorizedEvent {
                    event_type: event_type.to_string(),
                    contract_id: event
                        .event
                        .contract_id
                        .as_ref()
                        .map(|id| format!("{:?}", id)),
                    topics: topics.iter().map(|t| format!("{:?}", t)).collect(),
                    data: format!("{:?}", data_val),
                })
            } else {
                None
            }
        })
        .collect()
}

fn send_error(msg: String) {
    let res = SimulationResponse {
        status: "error".to_string(),
        error: Some(msg),
        events: vec![],
        diagnostic_events: vec![],
        categorized_events: vec![],
        logs: vec![],
        flamegraph: None,
        optimization_report: None,
        budget_usage: None,
    };
    println!("{}", serde_json::to_string(&res).unwrap());
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_time_travel_deserialization() {
        let json = r#"{"envelope_xdr": "AAAA", "result_meta_xdr": "BBBB", "timestamp": 1738077842, "ledger_sequence": 1234}"#;
        let req: SimulationRequest = serde_json::from_str(json).unwrap();
        assert_eq!(req.timestamp, Some(1738077842));
        assert_eq!(req.ledger_sequence, Some(1234));
    }
}

fn run_local_wasm_replay(wasm_path: &str, mock_args: &Option<Vec<String>>) {
    use soroban_env_host::{
        xdr::{ScAddress, ScSymbol, ScVal},
        Host,
    };
    use std::fs;

    eprintln!("🔧 Local WASM Replay Mode");
    eprintln!("WASM Path: {}", wasm_path);
    eprintln!("⚠️  WARNING: Using Mock State (not mainnet data)");
    eprintln!();

    // Read WASM file
    let wasm_bytes = match fs::read(wasm_path) {
        Ok(bytes) => {
            eprintln!("✓ Loaded WASM file: {} bytes", bytes.len());
            bytes
        }
        Err(e) => {
            return send_error(format!("Failed to read WASM file: {}", e));
        }
    };

    // Initialize Host
    let host = Host::default();
    host.set_diagnostic_level(soroban_env_host::DiagnosticLevel::Debug)
        .unwrap();

    eprintln!("✓ Initialized Host with diagnostic level: Debug");

    // TODO: Full execution requires 'testutils' feature which is currently causing build issues.
    // For now, we just parse args and print what we WOULD do.

    eprintln!(
        "⚠️  Full execution temporarily disabled due to build issues with 'testutils' feature."
    );
    eprintln!("   (See issue #183 for details)");

    // Parse Arguments (Mock)
    if let Some(args) = mock_args {
        if !args.is_empty() {
            eprintln!("▶ Would invoke function: {}", args[0]);
            eprintln!("  With args: {:?}", &args[1..]);
        }
    }

    // Capture Logs/Events
    let events = match host.get_events() {
        Ok(evs) => evs
            .0
            .iter()
            .map(|e| format!("{:?}", e))
            .collect::<Vec<String>>(),
        Err(e) => vec![format!("Failed to retrieve events: {:?}", e)],
    };

    let logs = vec![
        format!("Host Budget: {:?}", host.budget_cloned()),
        "Execution: Skipped (Build Issue)".to_string(),
    ];

    let response = SimulationResponse {
        status: "success".to_string(),
        error: None,
        events,
        diagnostic_events: vec![],
        categorized_events: vec![],
        logs,
        flamegraph: None,
        optimization_report: None,
        budget_usage: None,
    };

    println!("{}", serde_json::to_string(&response).unwrap());
}
