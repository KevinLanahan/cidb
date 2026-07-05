"use client";

import { useEffect, useState, useRef } from "react";
import { useParams } from "next/navigation";
import { supabase, Session, Step } from "@/lib/supabase";

const STATUS_CONFIG: Record<string, { label: string; color: string; bg: string; icon: string }> = {
  passed:  { label: "passed",  color: "#3fb950", bg: "#1a3a24", icon: "✓" },
  failed:  { label: "failed",  color: "#f85149", bg: "#3a1a1a", icon: "✗" },
  skipped: { label: "skipped", color: "#8b949e", bg: "#1c2128", icon: "⏭" },
  warned:  { label: "warned",  color: "#d29922", bg: "#3a2e1a", icon: "⚠" },
  aborted: { label: "aborted", color: "#8b949e", bg: "#1c2128", icon: "◼" },
  pending: { label: "pending", color: "#8b949e", bg: "#1c2128", icon: "·" },
  running: { label: "running", color: "#58a6ff", bg: "#1a2a3a", icon: "▶" },
};

const PLATFORM_CONFIG: Record<string, { label: string; color: string }> = {
  github:   { label: "GitHub Actions", color: "#58a6ff" },
  gitlab:   { label: "GitLab CI",      color: "#db6d28" },
  circleci: { label: "CircleCI",       color: "#3fb950" },
};

function StatusBadge({ status }: { status: Step["status"] }) {
  const cfg = STATUS_CONFIG[status] ?? STATUS_CONFIG.pending;
  const isRunning = status === "running";
  return (
    <span
      className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-mono font-medium"
      style={{
        color: cfg.color,
        background: cfg.bg,
        opacity: isRunning ? undefined : undefined,
      }}
    >
      {isRunning ? (
        <span style={{ display: "inline-block", animation: "spin 1s linear infinite" }}>◐</span>
      ) : cfg.icon}{" "}
      {cfg.label}
    </span>
  );
}

function StepAnalysis({ analysis }: { analysis: string }) {
  return (
    <div
      className="px-4 py-3 text-xs"
      style={{ borderTop: "1px solid #d2992240", background: "#1e1a0e" }}
    >
      <div className="flex items-center gap-1.5 mb-2 font-medium" style={{ color: "#d29922" }}>
        ✦ AI Analysis
      </div>
      <p style={{ color: "#e6edf3", lineHeight: "1.6", whiteSpace: "pre-wrap" }}>{analysis}</p>
    </div>
  );
}

function StepOutput({ output, status }: { output: string; status: Step["status"] }) {
  const borderColor = status === "failed" ? "#f8514940" : status === "warned" ? "#d2992240" : "#30363d";
  return (
    <details>
      <summary
        className="cursor-pointer text-xs px-4 py-1.5 select-none"
        style={{ color: "var(--gh-muted)", borderTop: "1px solid var(--gh-border)" }}
      >
        Show output
      </summary>
      <pre
        className="text-xs font-mono px-4 py-3 overflow-x-auto whitespace-pre-wrap break-words"
        style={{
          background: "#010409",
          color: "#c9d1d9",
          borderTop: `1px solid ${borderColor}`,
          maxHeight: "400px",
          overflowY: "auto",
        }}
      >
        {output}
      </pre>
    </details>
  );
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString("en-US", {
    month: "short", day: "numeric", year: "numeric",
    hour: "numeric", minute: "2-digit", hour12: true,
  });
}

function summary(steps: Step[]) {
  const counts: Record<string, number> = { passed: 0, failed: 0, skipped: 0, warned: 0, aborted: 0, pending: 0, running: 0 };
  for (const s of steps) counts[s.status] = (counts[s.status] ?? 0) + 1;
  return counts;
}

export default function SessionPage() {
  const params = useParams();
  const slug = params?.slug as string;

  const [session, setSession] = useState<Session | null>(null);
  const [notFound, setNotFound] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const missesRef = useRef(0);

  async function fetchSession() {
    const { data, error } = await supabase
      .from("sessions")
      .select("*")
      .eq("slug", slug)
      .single();

    if (error || !data) {
      missesRef.current += 1;
      // Only give up after 5 consecutive misses (~12 seconds) to handle timing/network blips.
      if (missesRef.current >= 5) setNotFound(true);
      return;
    }

    missesRef.current = 0;
    setSession(data as Session);

    // Stop polling once the session is done.
    if (data.session_status !== "running") {
      if (intervalRef.current) clearInterval(intervalRef.current);
    }
  }

  useEffect(() => {
    if (!slug) return;
    fetchSession();
    intervalRef.current = setInterval(fetchSession, 2500);
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, [slug]);

  if (notFound) {
    return (
      <main className="max-w-3xl mx-auto px-6 py-20 text-center">
        <p className="text-6xl mb-4">🔍</p>
        <h1 className="text-2xl font-bold mb-2" style={{ color: "var(--gh-text)" }}>Session not found</h1>
        <p className="text-sm" style={{ color: "var(--gh-muted)" }}>
          This session ID doesn&apos;t exist or may have been deleted.
        </p>
        <a href="/" className="inline-block mt-6 text-sm" style={{ color: "var(--gh-blue)" }}>← Back to lokal</a>
      </main>
    );
  }

  if (!session) {
    return (
      <main className="max-w-3xl mx-auto px-6 py-20 text-center">
        <p className="text-sm" style={{ color: "var(--gh-muted)" }}>Loading…</p>
      </main>
    );
  }

  const platform = PLATFORM_CONFIG[session.platform] ?? { label: session.platform, color: "#8b949e" };
  const counts = summary(session.steps);
  const isLive = session.session_status === "running";
  const overallPassed = session.session_status === "completed" && counts.failed === 0 && counts.aborted === 0;
  const overallFailed = session.session_status === "failed" || counts.failed > 0 || counts.aborted > 0;

  return (
    <>
      <style>{`
        @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.4; } }
      `}</style>
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
                {isLive && (
                  <span
                    className="text-xs font-medium px-2 py-0.5 rounded-full"
                    style={{ color: "#58a6ff", background: "#1a2a3a", border: "1px solid #58a6ff40", animation: "pulse 1.5s ease-in-out infinite" }}
                  >
                    ● LIVE
                  </span>
                )}
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
                color: isLive ? "#58a6ff" : overallPassed ? "#3fb950" : "#f85149",
                background: isLive ? "#1a2a3a" : overallPassed ? "#1a3a24" : "#3a1a1a",
              }}
            >
              {isLive ? "▶ Running…" : overallPassed ? "✓ All passed" : "✗ Failed"}
            </div>
          </div>

          {/* Summary counts */}
          <div className="flex gap-4 mt-5 pt-5 text-sm flex-wrap" style={{ borderTop: "1px solid var(--gh-border)" }}>
            {counts.passed  > 0 && <span style={{ color: "#3fb950" }}>✓ {counts.passed} passed</span>}
            {counts.failed  > 0 && <span style={{ color: "#f85149" }}>✗ {counts.failed} failed</span>}
            {counts.warned  > 0 && <span style={{ color: "#d29922" }}>⚠ {counts.warned} warned</span>}
            {counts.skipped > 0 && <span style={{ color: "#8b949e" }}>⏭ {counts.skipped} skipped</span>}
            {counts.aborted > 0 && <span style={{ color: "#8b949e" }}>◼ {counts.aborted} aborted</span>}
            {counts.pending > 0 && <span style={{ color: "#8b949e" }}>· {counts.pending} pending</span>}
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
              style={{
                borderBottom: i < session.steps.length - 1 ? "1px solid var(--gh-border)" : undefined,
                background: i % 2 === 0 ? "var(--gh-bg)" : "var(--gh-surface)",
              }}
            >
              <div className="flex items-center justify-between px-4 py-3 text-sm">
                <div className="flex items-center gap-3">
                  <span className="font-mono text-xs w-5 text-right" style={{ color: "var(--gh-muted)" }}>
                    {i + 1}
                  </span>
                  <span style={{ color: step.status === "pending" ? "var(--gh-muted)" : "var(--gh-text)" }}>
                    {step.name}
                  </span>
                </div>
                <StatusBadge status={step.status} />
              </div>
              {step.analysis && step.analysis.trim() && (
                <StepAnalysis analysis={step.analysis} />
              )}
              {step.output && step.output.trim() && (
                <StepOutput output={step.output} status={step.status} />
              )}
            </div>
          ))}
        </div>

        {/* Footer */}
        <p className="text-xs mt-6 text-center" style={{ color: "var(--gh-muted)" }}>
          Shared via{" "}
          <a href="/" style={{ color: "var(--gh-blue)" }}>lokal</a>
          {" "}· step-through CI debugger
        </p>
      </main>
    </>
  );
}
