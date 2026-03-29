pub mod latency;
pub mod models;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(i32)]
pub enum ExitCode {
    Ok = 0,
    Diffs = 1,
    Rules = 2,
    Invalid = 3,
    Runtime = 4,
}

impl From<ExitCode> for i32 {
    fn from(code: ExitCode) -> Self {
        code as i32
    }
}
