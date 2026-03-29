use anyhow::Result;
use rust_embed::Embed;
use std::collections::HashMap;
use tera::{Tera, Value};

#[derive(Embed)]
#[folder = "src/templates/"]
struct TemplatesAsset;

pub fn load_templates() -> Result<Tera> {
    let mut tera = Tera::default();

    for file in TemplatesAsset::iter() {
        let file_str = file.as_ref();
        if let Some(content) = TemplatesAsset::get(file_str) {
            let content_str = std::str::from_utf8(content.data.as_ref())?;
            tera.add_raw_template(file_str, content_str)?;
        }
    }

    register_functions(&mut tera);

    Ok(tera)
}

fn register_functions(tera: &mut Tera) {
    tera.register_function(
        "divFloat",
        |args: &HashMap<String, Value>| -> tera::Result<Value> {
            let a = args.get("a").and_then(|v| v.as_f64()).unwrap_or(0.0);
            let b = args.get("b").and_then(|v| v.as_f64()).unwrap_or(1.0);
            if b == 0.0 {
                return Ok(Value::Number(serde_json::Number::from(0)));
            }
            Ok(serde_json::json!(a / b))
        },
    );

    tera.register_function(
        "min_val",
        |args: &HashMap<String, Value>| -> tera::Result<Value> {
            let a = args.get("a").and_then(|v| v.as_i64()).unwrap_or(0);
            let b = args.get("b").and_then(|v| v.as_i64()).unwrap_or(0);
            Ok(Value::Number(serde_json::Number::from(if a < b {
                a
            } else {
                b
            })))
        },
    );

    tera.register_filter(
        "format_float",
        |value: &Value, args: &HashMap<String, Value>| -> tera::Result<Value> {
            let precision = args.get("precision").and_then(|v| v.as_u64()).unwrap_or(1) as usize;
            let num = value.as_f64().unwrap_or(0.0);
            Ok(Value::String(format!("{:.prec$}", num, prec = precision)))
        },
    );

    tera.register_filter(
        "date_format",
        |value: &Value, args: &HashMap<String, Value>| -> tera::Result<Value> {
            let format_str = args
                .get("format")
                .and_then(|v| v.as_str())
                .unwrap_or("%b %d, %H:%M");
            let date_str = value
                .as_str()
                .ok_or_else(|| tera::Error::msg("date_format expects a string"))?;
            match chrono::DateTime::parse_from_rfc3339(date_str) {
                Ok(dt) => Ok(Value::String(dt.format(format_str).to_string())),
                Err(_) => Ok(value.clone()),
            }
        },
    );
}
