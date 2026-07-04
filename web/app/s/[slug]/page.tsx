import { notFound } from "next/navigation";
import { supabase, Session, Step } from "@/lib/supabase";

const STATUS_CONFIG = {
  passed:  { label: "passed",  color: "#3fb950", bg: "#1a3a24", icon: "✓" },
  failed:  { label: "failed",  color: "#f85149", bg: "#3a1a1a", icon: "✗" },
  skipped: { label: "skipped", color: "#8b949e", bg: "#1c2128", icon: "⏭" },
  warned:  { label: "warned",  color: "#d29922", bg: "#3a2e1a", icon: "⚠" },
  aborted: { label: "aborted", color: "#8b949e", bg: "#1c2128", icon: "◼" },
} as const;

const PLATFORM_CONFIG: Record<string, { label: string; color: string }> = {
  github:   { label: "GitHub Actions", color: "#58a6ff" },
  gitlab:   { label: "GitLab CI",      color: "#db6d28" },
  circleci: { label: "CircleCI",       color: "#3fb950" },
};

function StatusBadge({ status }: { status: Step["status"] }) {
  const cfg = STATUS_CONFIG[status] ?? STATUS_CONFIG.skipped;
  return (
    <span
      className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-mono font-medium"
      style={{ color: cfg.color, background: cfg.bg }}
    >
      {cfg.icon} {cfg.label}
    </span>
  );
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString("en-US", {
    month: "short", day: "numeric", year: "numeric",
    hour: "numeric", minute: "2-digit", hour12: true,
  });
}

function summary(steps: Step[]) {
  const counts = { passed: 0, failed: 0, skipped: 0, warned: 0, aborted: 0 };
  for (const s of steps) counts[s.status] = (counts[s.status] ?? 0) + 1;
  return counts;
}

export default async function SessionPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;

  const { data, error } = await supabase
    .from("sessions")
    .select("*")
    .eq("slug", slug)
    .single();

  if (error || !data) notFound();

  const session = data as Session;
  const platform = PLATFORM_CONFIG[session.platform] ?? { label: session.platform, color: "#8b949e" };
  const counts = summary(session.steps);
  const overallPassed = counts.failed === 0 && counts.aborted === 0;

  return (
    <main className="max-w-3xl mx-auto px-6 py-10">
      {/* Header card */}
      <div className="rounded-lg p-6 mb-6" style={{ background: "var(--gh-surface)", border: "1px solid var(--gh-border)" }}>
        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div>
            <div className="flex items-center gap-2 mb-1">
              <span
                className="text-xs font-medium px-2 py-0.5 rounded-full"
                style={{ color: platform.color, background: "var(--gh-bg)", border: `1px solid ${platform.color}40` }}
              >
                {platform.label}
              </span>
            </div>
            <h1 className="text-2xl font-bold" style={{ color: "var(--gh-text)" }}>
              {session.workflow_name}
            </h1>
            <p className="text-sm mt-1" style={{ color: "var(--gh-muted)" }}>
              Session <span className="font-mono">{session.slug}</span> · {formatDate(session.created_at)}
            </p>
          </div>
          <div
            className="flex items-center gap-2 px-3 py-1.5 rounded-full text-sm font-semibold"
            style={{
              color: overallPassed ? "#3fb950" : "#f85149",
              background: overallPassed ? "#1a3a24" : "#3a1a1a",
            }}
          >
            {overallPassed ? "✓ All passed" : "✗ Failed"}
          </div>
        </div>

        {/* Summary counts */}
        <div className="flex gap-4 mt-5 pt-5 text-sm flex-wrap" style={{ borderTop: "1px solid var(--gh-border)" }}>
          {counts.passed  > 0 && <span style={{ color: "#3fb950" }}>✓ {counts.passed} passed</span>}
          {counts.failed  > 0 && <span style={{ color: "#f85149" }}>✗ {counts.failed} failed</span>}
          {counts.warned  > 0 && <span style={{ color: "#d29922" }}>⚠ {counts.warned} warned</span>}
          {counts.skipped > 0 && <span style={{ color: "#8b949e" }}>⏭ {counts.skipped} skipped</span>}
          {counts.aborted > 0 && <span style={{ color: "#8b949e" }}>◼ {counts.aborted} aborted</span>}
        </div>
      </div>

      {/* Steps */}
      <div className="rounded-lg overflow-hidden" style={{ border: "1px solid var(--gh-border)" }}>
        <div className="px-4 py-2.5 text-xs font-medium uppercase tracking-wide" style={{ background: "var(--gh-surface)", color: "var(--gh-muted)", borderBottom: "1px solid var(--gh-border)" }}>
          Steps — {session.steps.length} total
        </div>
        {session.steps.map((step, i) => (
          <div
            key={i}
            className="flex items-center justify-between px-4 py-3 text-sm"
            style={{
              borderBottom: i < session.steps.length - 1 ? "1px solid var(--gh-border)" : undefined,
              background: i % 2 === 0 ? "var(--gh-bg)" : "var(--gh-surface)",
            }}
          >
            <div className="flex items-center gap-3">
              <span className="font-mono text-xs w-5 text-right" style={{ color: "var(--gh-muted)" }}>
                {i + 1}
              </span>
              <span style={{ color: "var(--gh-text)" }}>{step.name}</span>
            </div>
            <StatusBadge status={step.status} />
          </div>
        ))}
      </div>

      {/* Footer */}
      <p className="text-xs mt-6 text-center" style={{ color: "var(--gh-muted)" }}>
        Shared via{" "}
        <a href="/" style={{ color: "var(--gh-blue)" }}>
          lokal
        </a>{" "}
        · step-through CI debugger
      </p>
    </main>
  );
}
