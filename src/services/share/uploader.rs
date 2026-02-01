// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

pub trait TraceUploader {
    fn upload(&self, content: &str, public: bool) -> Result<String, AppError>;
}
