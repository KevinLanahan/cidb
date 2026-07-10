import Anthropic from "@anthropic-ai/sdk";
import { NextRequest, NextResponse } from "next/server";

const client = new Anthropic({ apiKey: process.env.ANTHROPIC_API_KEY });

export async function POST(req: NextRequest) {
  const { command, output, exitCode } = await req.json();

  if (!command || exitCode === undefined) {
    return NextResponse.json({ error: "missing fields" }, { status: 400 });
  }

  const prompt = `A CI pipeline step failed. Explain in 2-3 sentences why it failed and how to fix it. Be specific and practical — no fluff.

Command:
${command}

Output:
${output ?? "(no output)"}

Exit code: ${exitCode}`;

  const msg = await client.messages.create({
    model: "claude-haiku-4-5-20251001",
    max_tokens: 300,
    messages: [{ role: "user", content: prompt }],
  });

  const text = msg.content
    .filter((b) => b.type === "text")
    .map((b) => b.text)
    .join("");

  return NextResponse.json({ analysis: text });
}
