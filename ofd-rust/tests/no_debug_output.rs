use std::fs;
use std::path::{Path, PathBuf};

#[test]
fn tracked_text_sources_do_not_contain_diagnostic_output() {
    let mut files = Vec::new();
    collect_files(Path::new("."), &mut files);

    let mut violations = Vec::new();
    for path in files {
        let Ok(source) = fs::read_to_string(&path) else {
            continue;
        };

        for (line_index, line) in source.lines().enumerate() {
            for pattern in forbidden_patterns() {
                if line.contains(pattern) {
                    violations.push(format!(
                        "{}:{} contains {}",
                        path.display(),
                        line_index + 1,
                        pattern
                    ));
                }
            }
        }
    }

    assert!(
        violations.is_empty(),
        "tracked text sources must not contain diagnostic output statements:\n{}",
        violations.join("\n")
    );
}

fn forbidden_patterns() -> Vec<&'static str> {
    vec![
        concat!("console", "."),
        concat!("debug", "ger"),
        concat!("debug", "_log"),
        concat!("debug", "_warn"),
        concat!("print", "ln!"),
        concat!("eprint", "ln!"),
        concat!("dbg", "!"),
    ]
}

fn collect_files(dir: &Path, files: &mut Vec<PathBuf>) {
    let Ok(entries) = fs::read_dir(dir) else {
        return;
    };

    for entry in entries.flatten() {
        let path = entry.path();
        if should_skip(&path) {
            continue;
        }
        if path.is_dir() {
            collect_files(&path, files);
        } else if matches!(
            path.extension().and_then(|ext| ext.to_str()),
            Some(
                "rs" | "js" | "mjs" | "html" | "md" | "css" | "json" | "toml" | "sh" | "bat"
            )
        ) {
            files.push(path);
        }
    }
}

fn should_skip(path: &Path) -> bool {
    path.components().any(|component| {
        let name = component.as_os_str().to_string_lossy();
        matches!(
            name.as_ref(),
            ".git" | "target" | "node_modules" | ".DS_Store"
        )
    })
}
