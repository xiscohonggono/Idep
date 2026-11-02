import { AnalogyStudySidebar } from '@/components/AnalogyStudySidebar';
import {
  createChapterExplanation,
  elaborateExplanation,
  getUser,
  listHighlightExplanations,
  upsertExplanationFeedback,
  type AIExplanation,
  type ElaborateBody,
  type FeedbackBody,
} from '@/lib/api';

type HomePageProps = {
  searchParams: Promise<{ user?: string; highlight?: string }>;
};

// shortcut: the study sidebar is reached through ?user=&highlight= with a client-asserted user ID, move to the chapter reader with the Stage 1 session
async function StudySidebar({ userId, highlightId }: { userId: string; highlightId: string }) {
  let explanations: AIExplanation[];
  try {
    explanations = await listHighlightExplanations(userId, highlightId);
  } catch (err) {
    return <p role="alert">{err instanceof Error ? err.message : String(err)}</p>;
  }

  // Server actions keep the API calls server-side, because the Go API sets no CORS headers.
  async function compare(jobFieldCode: string, current: AIExplanation) {
    'use server';
    return createChapterExplanation(userId, current.chapter_id, {
      version_id: current.chapter_version_id,
      selected_text: current.selected_text,
      start_offset: current.start_offset,
      end_offset: current.end_offset,
      job_field_code: jobFieldCode,
    });
  }

  async function elaborate(explanationId: string, body: ElaborateBody) {
    'use server';
    return elaborateExplanation(userId, explanationId, body);
  }

  async function feedback(explanationId: string, body: FeedbackBody) {
    'use server';
    return upsertExplanationFeedback(userId, explanationId, body);
  }

  return (
    <AnalogyStudySidebar
      explanations={explanations}
      onCompare={compare}
      onElaborate={elaborate}
      onFeedback={feedback}
    />
  );
}

export default async function HomePage({ searchParams }: HomePageProps) {
  const { user, highlight } = await searchParams;

  let result: string;
  try {
    result = JSON.stringify(await getUser('00000000-0000-0000-0000-000000000101'), null, 2);
  } catch (err) {
    result = err instanceof Error ? err.message : String(err);
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 font-sans">
      <h1 className="text-2xl font-bold">Go API Demo</h1>
      <pre className="rounded-md bg-gray-100 text-sm text-gray-900">{result}</pre>
      {user && highlight && <StudySidebar userId={user} highlightId={highlight} />}
    </main>
  );
}
