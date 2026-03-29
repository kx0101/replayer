use indicatif::{ProgressBar as IndicatifBar, ProgressStyle};

pub struct ProgressBar {
    bar: IndicatifBar,
}

impl ProgressBar {
    pub fn new(total: u64) -> Self {
        let bar = IndicatifBar::new(total);
        bar.set_style(
            ProgressStyle::default_bar()
                .template("[{bar:50}] {pos}/{len} ({percent}%) | Elapsed: {elapsed} | ETA: {eta}")
                .unwrap()
                .progress_chars("█░"),
        );
        Self { bar }
    }

    pub fn increment(&self) {
        self.bar.inc(1);
    }

    pub fn finish(&self) {
        self.bar.finish();
    }
}
