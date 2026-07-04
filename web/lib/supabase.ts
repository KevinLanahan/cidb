import { createClient } from "@supabase/supabase-js";

export const supabase = createClient(
  process.env.NEXT_PUBLIC_SUPABASE_URL!,
  process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!
);

export type Step = {
  name: string;
  status: "passed" | "failed" | "skipped" | "warned" | "aborted";
};

export type Session = {
  id: string;
  slug: string;
  workflow_name: string;
  platform: string;
  steps: Step[];
  created_at: string;
};
