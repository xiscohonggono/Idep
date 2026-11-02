import { renderToStaticMarkup } from 'react-dom/server';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AnalogyStudySidebar } from '../components/AnalogyStudySidebar';
import {
  createChapterExplanation,
  elaborateExplanation,
  listHighlightExplanations,
  upsertExplanationFeedback,
  type AIElaboration,
  type AIExplanation,
  type ExplanationFeedback,
} from '../lib/api/go';

const userId = '00000000-0000-0000-0000-000000000101';
const id = '00000000-0000-0000-0000-000000000201';

const elaboration: AIElaboration = {
  id,
  explanation_id: id,
  target_section: 'step',
  target_index: 0,
  is_on_topic: false,
  hint_text: 'Try asking about the passage.',
  model_identifier: 'gpt-4.1-mini',
  created_at: '2026-11-03T00:00:00Z',
};

const feedback: ExplanationFeedback = {
  id,
  explanation_id: id,
  user_id: userId,
  chapter_id: id,
  job_field_code: 'fnb',
  rating: 'dislike',
  comment: 'We use a flat grill.',
  created_at: '2026-11-03T00:00:00Z',
  updated_at: '2026-11-03T00:00:00Z',
};

const explanation: AIExplanation = {
  id,
  highlight_id: id,
  user_id: userId,
  chapter_id: id,
  chapter_version_id: id,
  start_offset: 0,
  end_offset: 33,
  job_field_code: 'fnb',
  role_title_snapshot: 'line cook',
  is_everyday_fallback: false,
  selected_text: 'Plants use sunlight to make food.',
  terms: [
    { term: 'Chlorophyll', definition: 'The green part of a leaf.', linked_chapter_slug: 'plant-cells' },
    { term: 'Glucose', definition: 'A simple sugar.' },
  ],
  analogy_steps: [
    { step_number: 1, work_element: 'The oven', subject_term: 'The leaf', explanation: 'Heat becomes a meal.' },
  ],
  analogy_limits: [
    { analogy_limit: 'Ovens need gas', accurate_explanation: 'Leaves only need light.' },
  ],
  prerequisites: [{ chapter_id: id, slug: 'energy', title: 'What Energy Is', reason: 'Needed for step 1.' }],
  elaborations: [elaboration],
  user_feedback: feedback,
  model_identifier: 'gpt-4.1-mini',
  prompt_version: 'grounded-analogy-v1',
  created_at: '2026-11-03T00:00:00Z',
};

function stubFetch(body: unknown, status = 200) {
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status }));
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

describe('analogy API client', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
  });

  it('creates an explanation with the caller header and JSON body', async () => {
    vi.stubEnv('NEXT_PUBLIC_API_URL', 'https://api.test');
    const fetchMock = stubFetch(explanation, 201);
    const body = { version_id: id, selected_text: 'text', start_offset: 0, end_offset: 4, job_field_code: 'fnb' };

    await expect(createChapterExplanation(userId, 'a/b', body)).resolves.toEqual(explanation);

    expect(fetchMock).toHaveBeenCalledWith('https://api.test/api/v1/chapters/a%2Fb/explanations', {
      method: 'POST',
      cache: 'no-store',
      headers: { 'X-User-ID': userId, 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  });

  it('lists explanations without sending a body', async () => {
    vi.stubEnv('NEXT_PUBLIC_API_URL', 'https://api.test');
    const fetchMock = stubFetch({ explanations: [explanation] });

    await expect(listHighlightExplanations(userId, id)).resolves.toEqual([explanation]);

    expect(fetchMock).toHaveBeenCalledWith(`https://api.test/api/v1/highlights/${id}/explanations`, {
      method: 'GET',
      cache: 'no-store',
      headers: { 'X-User-ID': userId },
    });
  });

  it('requests an elaboration', async () => {
    vi.stubEnv('NEXT_PUBLIC_API_URL', 'https://api.test');
    const fetchMock = stubFetch(elaboration);

    await expect(
      elaborateExplanation(userId, id, { target_section: 'step', target_index: 0 }),
    ).resolves.toEqual(elaboration);

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(`https://api.test/api/v1/explanations/${id}/elaborate`);
    expect(init.method).toBe('POST');
  });

  it('upserts feedback with PUT', async () => {
    vi.stubEnv('NEXT_PUBLIC_API_URL', 'https://api.test');
    const fetchMock = stubFetch(feedback);

    await expect(
      upsertExplanationFeedback(userId, id, { rating: 'dislike', comment: 'Off' }),
    ).resolves.toEqual(feedback);

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(`https://api.test/api/v1/explanations/${id}/feedback`);
    expect(init.method).toBe('PUT');
    expect(init.body).toBe(JSON.stringify({ rating: 'dislike', comment: 'Off' }));
  });

  it.each([401, 403, 429, 503])('throws on %i', async (status) => {
    stubFetch({ title: 'error' }, status);

    await expect(listHighlightExplanations(userId, id)).rejects.toThrow(String(status));
  });
});

describe('AnalogyStudySidebar', () => {
  const render = (explanations: AIExplanation[]) =>
    renderToStaticMarkup(
      <AnalogyStudySidebar
        explanations={explanations}
        onCompare={vi.fn()}
        onElaborate={vi.fn()}
        onFeedback={vi.fn()}
      />,
    );

  it('renders the passage, terms, steps, limits, and prerequisites', () => {
    const html = render([explanation]);

    expect(html).toContain('Plants use sunlight to make food.');
    expect(html).toContain('Key terms');
    expect(html).toContain('href="/chapters/plant-cells"');
    expect(html).toContain('Glucose');
    expect(html).toContain('The oven');
    expect(html).toContain('The leaf');
    expect(html).toContain('Where this comparison falls short');
    expect(html).toContain('Ovens need gas');
    expect(html).toContain('Master these first');
    expect(html).toContain('href="/chapters/energy"');
  });

  it('shows the profession and comparison pills', () => {
    const html = render([explanation]);

    expect(html).toContain('Food &amp; beverage, line cook');
    expect(html).toContain('aria-label="Compare with another profession"');
    expect(html).toMatch(/aria-pressed="true"[^>]*>Food &amp; beverage</);
    expect(html).toMatch(/aria-pressed="false"[^>]*>Construction</);
    expect(html).toMatch(/aria-pressed="false"[^>]*>Healthcare</);
  });

  it('labels the everyday fallback', () => {
    const html = render([
      { ...explanation, job_field_code: undefined, role_title_snapshot: undefined, is_everyday_fallback: true },
    ]);

    expect(html).toContain('data-testid="profession-badge">Everyday Life');
  });

  it('offers an elaboration drawer on each term, step, and limit', () => {
    const html = render([explanation]);

    expect(html.match(/data-testid="elaboration-drawer"/g)).toHaveLength(4);
    expect(html).toContain('Give a concrete workplace example');
    expect(html).toContain('Explain simpler');
    expect(html).toContain('maxLength="300"');
    expect(html).toContain('Try asking about the passage.');
  });

  it('restores saved dislike feedback with its prompt and comment', () => {
    const html = render([explanation]);

    expect(html).toMatch(/aria-pressed="true"[^>]*>👎/);
    expect(html).toMatch(/aria-pressed="false"[^>]*>👍/);
    expect(html).toContain('What felt inaccurate for your job?');
    expect(html).toContain('We use a flat grill.');
    expect(html).toContain('maxLength="500"');
  });

  it('asks what clicked after a like', () => {
    const html = render([
      { ...explanation, user_feedback: { ...feedback, rating: 'like', comment: undefined } },
    ]);

    expect(html).toContain('What clicked about this analogy?');
  });

  it('hides the comment box before a rating is chosen', () => {
    const html = render([{ ...explanation, user_feedback: undefined }]);

    expect(html).not.toContain('<textarea');
  });

  it('shows direction when nothing has been explained yet', () => {
    expect(render([])).toContain('Highlight up to three paragraphs');
  });
});
