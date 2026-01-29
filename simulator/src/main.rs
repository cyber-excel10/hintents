// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

mod theme;
mod config;
mod cli;
mod ipc;
mod gas_optimizer;

use base64::Engine as _;
use serde::{Deserialize, Serialize};
use soroban_env_host::xdr::ReadXdr;
use std::collections::HashMap;
use std::io::{self, Read};
use std::panic;

use gas_optimizer::{BudgetMetrics, GasOptimizationAdvisor, OptimizationReport};

#[derive(Debug, Deserialize)]
struct SimulationRequest {
    envelope_xdr: String,
    result_meta_xdr: String,
    // Key XDR -> Entry XDR
    ledger_entries: Option<HashMap<String, String>>,
    // Optional: Path to local WASM file for local replay
    wasm_path: Option<String>,
    // Optional: Mock arguments for local replay (JSON array of strings)
    mock_args: Option<Vec<String>>,
    profile: Option<bool>,
    #[serde(default)]
    enable_optimization_advisor: bool,
}

#[derive(Debug, Serialize)]
struct SimulationResponse {
    status: String,
    error: Option<String>,
    events: Vec<String>,
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

        }
    };

    // Check if this is a local WASM replay (no network data)
    if let Some(wasm_path) = &request.wasm_path {
        return run_local_wasm_replay(wasm_path, &request.mock_args);
    }

    // Decode Envelope XDR
    let envelope = match base64::engine::general_purpose::STANDARD.decode(&request.envelope_xdr) {
        Ok(bytes) => match soroban_env_host::xdr::TransactionEnvelope::from_xdr(

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

    // Populate Host Storage
    let mut loaded_entries_count = 0;
    if let Some(entries) = &request.ledger_entries {
        for (key_xdr, entry_xdr) in entries {

                    Ok(k) => k,
                    Err(e) => return send_error(format!("Failed to parse LedgerKey XDR: {}", e)),
                },
                Err(e) => return send_error(format!("Failed to decode LedgerKey Base64: {}", e)),
            };


                    Ok(e) => e,
                    Err(e) => return send_error(format!("Failed to parse LedgerEntry XDR: {}", e)),
                },
                Err(e) => return send_error(format!("Failed to decode LedgerEntry Base64: {}", e)),
            };

            loaded_entries_count += 1;
        }
    }


    let operations = match &envelope {
        soroban_env_host::xdr::TransactionEnvelope::Tx(tx_v1) => &tx_v1.tx.operations,
        soroban_env_host::xdr::TransactionEnvelope::TxV0(tx_v0) => &tx_v0.tx.operations,
        soroban_env_host::xdr::TransactionEnvelope::TxFeeBump(bump) => match &bump.tx.inner_tx {
            soroban_env_host::xdr::FeeBumpTransactionInnerTx::Tx(tx_v1) => &tx_v1.tx.operations,
        },
    };


            ];
            final_logs.extend(exec_logs);

            let response = SimulationResponse {
                status: "success".to_string(),
                error: None,
                events,
                logs: final_logs,
                flamegraph: flamegraph_svg,
                optimization_report,
                budget_usage: Some(budget_usage),
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
) -> Vec<String> {
    let mut logs = vec![];
    for (i, op) in operations.as_slice().iter().enumerate() {
        logs.push(format!("Processing operation {}: {:?}", i, op.body));
        // Placeholder for real host invocation
    }
    logs
}

/// Decodes generic WASM traps into human-readable messages.
fn decode_wasm_trap(err: &soroban_env_host::HostError) -> String {
    let err_str = format!("{:?}", err);
    let err_lower = err_str.to_lowercase();

    // Check for VM-initiated traps
    if err_lower.contains("wasm trap") {
        if err_lower.contains("unreachable") {
            return "Unreachable Instruction: The contract hit a panic or unreachable code path.".to_string();
        }
        if err_lower.contains("out of bounds") {
            return "Out of Bounds Access: The contract tried to access invalid memory (OOB).".to_string();
        }
        if err_lower.contains("integer overflow") {
            return "Integer Overflow: A mathematical operation exceeded the type limits.".to_string();
        }
        if err_lower.contains("stack overflow") {
            return "Stack Overflow: The contract's recursion or stack usage is too high.".to_string();
        }
        if err_lower.contains("divide by zero") {
            return "Division by Zero: The contract attempted to divide by zero.".to_string();
        }
        return format!("Wasm Trap: {}", err_str);
    }

    // Differentiate Host-initiated traps
    if err_str.contains("HostError") {
        return format!("Host-initiated Trap: {}", err_str);
    }

    format!("Execution Error: {}", err_str)
}

fn send_error(msg: String) {
    let res = SimulationResponse {
        status: "error".to_string(),
        error: Some(msg),
        events: vec![],
        logs: vec![],
        flamegraph: None,
        optimization_report: None,
        budget_usage: None,
    };
    println!("{}", serde_json::to_string(&res).unwrap());

}