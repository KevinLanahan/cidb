export default function Home() {
  return (
    <main className="max-w-3xl mx-auto px-6 py-20">
      <div className="mb-10">
        <h1 className="text-4xl font-bold mb-4" style={{ color: "var(--gh-text)" }}>
          Debug CI pipelines locally.
        </h1>
        <p className="text-lg" style={{ color: "var(--gh-muted)" }}>
          lokal runs your GitHub Actions, GitLab CI, or CircleCI pipeline in Docker —
          pausing before each step so you can inspect, skip, retry, or drop into a live shell.
        </p>
      </div>

      <div className="rounded-lg p-5 mb-10 font-mono text-sm" style={{ background: "var(--gh-surface)", border: "1px solid var(--gh-border)" }}>
        <p style={{ color: "var(--gh-muted)" }} className="mb-3 text-xs uppercase tracking-wide font-sans">Install</p>
        <p style={{ color: "var(--gh-green)" }}>$ git clone https://github.com/KevinLanahan/Lokal.git</p>
        <p style={{ color: "var(--gh-green)" }}>$ cd Lokal && go build -o lokal .</p>
        <p style={{ color: "var(--gh-green)" }}>$ ./lokal run</p>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        {[
          { label: "GitHub Actions", icon: "⚡" },
          { label: "GitLab CI", icon: "🦊" },
          { label: "CircleCI", icon: "⭕" },
        ].map((p) => (
          <div key={p.label} className="rounded-lg p-4 text-sm" style={{ background: "var(--gh-surface)", border: "1px solid var(--gh-border)" }}>
            <span className="mr-2">{p.icon}</span>
            <span style={{ color: "var(--gh-text)" }}>{p.label}</span>
          </div>
        ))}
      </div>

      <div className="mt-10">
        <a
          href="https://github.com/KevinLanahan/Lokal"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors"
          style={{ background: "var(--gh-surface)", border: "1px solid var(--gh-border)", color: "var(--gh-text)" }}
        >
          View on GitHub
        </a>
      </div>
    </main>
  );
}
