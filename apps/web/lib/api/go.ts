import type { paths } from '@/lib/api/schema';

export type User =
  paths['/api/v1/users/{id}']['get']['responses'][200]['content']['application/json'];

export async function getUser(id: string): Promise<User> {
  const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';
  const res = await fetch(`${baseUrl}/api/v1/users/${encodeURIComponent(id)}`, {
    cache: 'no-store',
  });

  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }

  return res.json();
}

type CreateExplanationPath = paths['/api/v1/chapters/{chapter_id}/explanations']['post'];
type ElaboratePath = paths['/api/v1/explanations/{explanation_id}/elaborate']['post'];
type FeedbackPath = paths['/api/v1/explanations/{explanation_id}/feedback']['put'];

export type AIExplanation = CreateExplanationPath['responses'][201]['content']['application/json'];
export type AIElaboration = ElaboratePath['responses'][200]['content']['application/json'];
export type ExplanationFeedback = FeedbackPath['responses'][200]['content']['application/json'];
export type CreateExplanationBody = CreateExplanationPath['requestBody']['content']['application/json'];
export type ElaborateBody = ElaboratePath['requestBody']['content']['application/json'];
export type FeedbackBody = FeedbackPath['requestBody']['content']['application/json'];

async function send<T>(path: string, userId: string, method: string, body?: unknown): Promise<T> {
  const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';
  const res = await fetch(`${baseUrl}${path}`, {
    method,
    cache: 'no-store',
    headers: {
      'X-User-ID': userId,
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });

  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }

  return res.json();
}

export function createChapterExplanation(
  userId: string,
  chapterId: string,
  body: CreateExplanationBody,
): Promise<AIExplanation> {
  return send(`/api/v1/chapters/${encodeURIComponent(chapterId)}/explanations`, userId, 'POST', body);
}

export async function listHighlightExplanations(
  userId: string,
  highlightId: string,
): Promise<AIExplanation[]> {
  const { explanations } = await send<
    paths['/api/v1/highlights/{highlight_id}/explanations']['get']['responses'][200]['content']['application/json']
  >(`/api/v1/highlights/${encodeURIComponent(highlightId)}/explanations`, userId, 'GET');
  return explanations;
}

export function elaborateExplanation(
  userId: string,
  explanationId: string,
  body: ElaborateBody,
): Promise<AIElaboration> {
  return send(`/api/v1/explanations/${encodeURIComponent(explanationId)}/elaborate`, userId, 'POST', body);
}

export function upsertExplanationFeedback(
  userId: string,
  explanationId: string,
  body: FeedbackBody,
): Promise<ExplanationFeedback> {
  return send(`/api/v1/explanations/${encodeURIComponent(explanationId)}/feedback`, userId, 'PUT', body);
}
