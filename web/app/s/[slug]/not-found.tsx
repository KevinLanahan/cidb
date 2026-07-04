export default function NotFound() {
  return (
    <main className="max-w-3xl mx-auto px-6 py-20 text-center">
      <p className="text-6xl mb-4">🔍</p>
      <h1 className="text-2xl font-bold mb-2" style={{ color: "var(--gh-text)" }}>
        Session not found
      </h1>
      <p className="text-sm" style={{ color: "var(--gh-muted)" }}>
        This session ID doesn&apos;t exist or may have been deleted.
      </p>
      <a href="/" className="inline-block mt-6 text-sm" style={{ color: "var(--gh-blue)" }}>
        ← Back to lokal
      </a>
    </main>
  );
}
