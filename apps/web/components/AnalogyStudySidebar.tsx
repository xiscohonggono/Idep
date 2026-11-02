'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import type {
  AIElaboration,
  AIExplanation,
  ElaborateBody,
  ExplanationFeedback,
  FeedbackBody,
} from '@/lib/api';

// Mirrors the job_fields seed: the API has no job-field catalog route yet.
const JOB_FIELDS = [
  { code: 'fnb', label: 'Food & beverage' },
  { code: 'retail', label: 'Retail' },
  { code: 'logistics', label: 'Logistics' },
  { code: 'healthcare', label: 'Healthcare' },
  { code: 'construction', label: 'Construction' },
  { code: 'hospitality', label: 'Hospitality' },
  { code: 'manufacturing', label: 'Manufacturing' },
  { code: 'office_admin', label: 'Office' },
  { code: 'automotive', label: 'Automotive' },
  { code: 'childcare_education', label: 'Childcare' },
];

const QUICK_PROMPTS = ['Give a concrete workplace example', 'Explain simpler'];
const MAX_PROMPT = 300;
const MAX_COMMENT = 500;

type Section = ElaborateBody['target_section'];
type Rating = FeedbackBody['rating'];

export type AnalogyStudySidebarProps = {
  explanations: AIExplanation[];
  onCompare: (jobFieldCode: string, current: AIExplanation) => Promise<AIExplanation>;
  onElaborate: (explanationId: string, body: ElaborateBody) => Promise<AIElaboration>;
  onFeedback: (explanationId: string, body: FeedbackBody) => Promise<ExplanationFeedback>;
};

function fieldLabel(code: string) {
  return JOB_FIELDS.find((field) => field.code === code)?.label ?? code;
}

function professionLabel(explanation: AIExplanation) {
  if (explanation.is_everyday_fallback || !explanation.job_field_code) return 'Everyday Life';
  const field = fieldLabel(explanation.job_field_code);
  return explanation.role_title_snapshot ? `${field}, ${explanation.role_title_snapshot}` : field;
}

const focusRing =
  'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#0f5c52]';
const buttonBase = `cursor-pointer rounded-md px-3 py-1.5 text-sm disabled:cursor-wait disabled:opacity-60 ${focusRing}`;

function SectionBlock({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mt-7">
      <h3 className="mb-2 text-base font-semibold text-[#0f3d37]">{title}</h3>
      {children}
    </section>
  );
}

function ElaborationDrawer(props: {
  explanationId: string;
  section: Section;
  index?: number;
  elaborations: AIElaboration[];
  onElaborate: AnalogyStudySidebarProps['onElaborate'];
  onAdded: (elaboration: AIElaboration) => void;
}) {
  const { explanationId, section, index, elaborations, onElaborate, onAdded } = props;
  const [prompt, setPrompt] = useState('');
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);

  const answers = elaborations.filter(
    (item) => item.target_section === section && item.target_index === index,
  );

  async function ask(text: string) {
    setBusy(true);
    setFailed(false);
    try {
      const trimmed = text.trim();
      onAdded(
        await onElaborate(explanationId, {
          target_section: section,
          ...(index === undefined ? {} : { target_index: index }),
          ...(trimmed ? { user_prompt: trimmed } : {}),
        }),
      );
      setPrompt('');
    } catch {
      setFailed(true);
    } finally {
      setBusy(false);
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    void ask(prompt);
  }

  return (
    <details className="mt-2 text-sm" data-testid="elaboration-drawer">
      <summary className={`w-fit cursor-pointer text-[#0f5c52] underline underline-offset-2 ${focusRing}`}>
        Ask for more details
      </summary>
      <div className="mt-2 border-l-2 border-[#b9d3cc] pl-3">
        {answers.map((answer) => (
          <div key={answer.id} className="mb-3 whitespace-pre-wrap">
            {answer.is_on_topic ? (
              <>
                <p>{answer.elaboration_markdown}</p>
                {answer.work_example && <p className="mt-1 text-[#35524c]">{answer.work_example}</p>}
              </>
            ) : (
              <p className="text-[#35524c]">{answer.hint_text}</p>
            )}
          </div>
        ))}
        <div className="flex flex-wrap gap-2">
          {QUICK_PROMPTS.map((quick) => (
            <button
              key={quick}
              type="button"
              disabled={busy}
              onClick={() => void ask(quick)}
              className={`${buttonBase} bg-[#e3eeeb] text-[#0f3d37] hover:bg-[#d3e4df]`}
            >
              {quick}
            </button>
          ))}
        </div>
        <form onSubmit={submit} className="mt-2 flex gap-2">
          <input
            aria-label="Your question about this part"
            value={prompt}
            maxLength={MAX_PROMPT}
            onChange={(event) => setPrompt(event.target.value)}
            placeholder="Ask about this part"
            className={`min-w-0 flex-1 rounded-md border border-[#9fbdb5] bg-white px-2 py-1.5 ${focusRing}`}
          />
          <button type="submit" disabled={busy} className={`${buttonBase} bg-[#0f5c52] text-white`}>
            {busy ? 'Asking' : 'Ask'}
          </button>
        </form>
        {failed && (
          <p role="alert" className="mt-2 text-[#9b2c1f]">
            That question didn&apos;t go through. Try again in a moment.
          </p>
        )}
      </div>
    </details>
  );
}

function FeedbackControls(props: {
  explanation: AIExplanation;
  onFeedback: AnalogyStudySidebarProps['onFeedback'];
  onSaved: (feedback: ExplanationFeedback) => void;
}) {
  const { explanation, onFeedback, onSaved } = props;
  const saved = explanation.user_feedback;
  const [rating, setRating] = useState<Rating | undefined>(saved?.rating);
  const [comment, setComment] = useState(saved?.comment ?? '');
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!rating) return;
    setBusy(true);
    setFailed(false);
    try {
      const trimmed = comment.trim();
      onSaved(await onFeedback(explanation.id, { rating, ...(trimmed ? { comment: trimmed } : {}) }));
    } catch {
      setFailed(true);
    } finally {
      setBusy(false);
    }
  }

  const ratingButton = (value: Rating, emoji: string, label: string) => (
    <button
      type="button"
      aria-pressed={rating === value}
      aria-label={label}
      onClick={() => setRating(value)}
      className={`${buttonBase} border ${
        rating === value ? 'border-[#0f5c52] bg-[#d3e4df]' : 'border-[#9fbdb5] bg-white'
      }`}
    >
      {emoji}
    </button>
  );

  return (
    <SectionBlock title="Did this analogy help?">
      <form onSubmit={submit}>
        <div className="flex gap-2">
          {ratingButton('like', '👍', 'Like this analogy')}
          {ratingButton('dislike', '👎', 'Dislike this analogy')}
        </div>
        {rating && (
          <>
            <label className="mt-3 block text-sm">
              {rating === 'like' ? 'What clicked about this analogy?' : 'What felt inaccurate for your job?'}
              <textarea
                value={comment}
                maxLength={MAX_COMMENT}
                rows={2}
                onChange={(event) => setComment(event.target.value)}
                className={`mt-1 block w-full rounded-md border border-[#9fbdb5] bg-white px-2 py-1.5 ${focusRing}`}
              />
            </label>
            <button type="submit" disabled={busy} className={`${buttonBase} mt-2 bg-[#0f5c52] text-white`}>
              {busy ? 'Saving' : 'Save feedback'}
            </button>
          </>
        )}
        {saved?.rating === rating && (saved?.comment ?? '') === comment.trim() && !busy && !failed && (
          <p role="status" className="mt-2 text-sm text-[#35524c]">
            Feedback saved.
          </p>
        )}
        {failed && (
          <p role="alert" className="mt-2 text-sm text-[#9b2c1f]">
            Your feedback wasn&apos;t saved. Try again in a moment.
          </p>
        )}
      </form>
    </SectionBlock>
  );
}

export function AnalogyStudySidebar({
  explanations: initial,
  onCompare,
  onElaborate,
  onFeedback,
}: AnalogyStudySidebarProps) {
  const [explanations, setExplanations] = useState(initial);
  const [activeId, setActiveId] = useState(initial[0]?.id);
  const [pending, setPending] = useState<string>();
  const [failedField, setFailedField] = useState<string>();

  const active = explanations.find((item) => item.id === activeId);
  if (!active) {
    return (
      <aside className="w-full max-w-xl rounded-lg bg-[#f2f7f5] p-5 text-[#17231f]">
        <p>Highlight up to three paragraphs in a chapter to see them explained through your work.</p>
      </aside>
    );
  }

  async function selectField(code: string) {
    if (!active) return;
    const existing = explanations.find((item) => item.job_field_code === code);
    if (existing) {
      setActiveId(existing.id);
      return;
    }

    setPending(code);
    setFailedField(undefined);
    try {
      const created = await onCompare(code, active);
      setExplanations((items) => [...items, created]);
      setActiveId(created.id);
    } catch {
      setFailedField(code);
    } finally {
      setPending(undefined);
    }
  }

  function addElaboration(elaboration: AIElaboration) {
    setExplanations((items) =>
      items.map((item) =>
        item.id === elaboration.explanation_id
          ? { ...item, elaborations: [...item.elaborations, elaboration] }
          : item,
      ),
    );
  }

  function saveFeedback(feedback: ExplanationFeedback) {
    setExplanations((items) =>
      items.map((item) =>
        item.id === feedback.explanation_id ? { ...item, user_feedback: feedback } : item,
      ),
    );
  }

  const drawer = (section: Section, index?: number) => (
    <ElaborationDrawer
      explanationId={active.id}
      section={section}
      index={index}
      elaborations={active.elaborations}
      onElaborate={onElaborate}
      onAdded={addElaboration}
    />
  );

  const everyday = explanations.find((item) => item.is_everyday_fallback);
  const pillClass = (selected: boolean) =>
    `${buttonBase} rounded-full border ${
      selected ? 'border-[#0f5c52] bg-[#0f5c52] text-white' : 'border-[#9fbdb5] bg-white text-[#0f3d37]'
    }`;

  return (
    <aside
      aria-label="Explanation of your highlight"
      className="w-full max-w-xl rounded-lg bg-[#f2f7f5] p-5 leading-relaxed text-[#17231f]"
    >
      <blockquote className="font-serif text-lg leading-loose">
        <mark className="bg-[#fbe96b] box-decoration-clone px-1 text-[#17231f]">{active.selected_text}</mark>
      </blockquote>

      <p className="mt-4 text-sm">
        Explained for <strong data-testid="profession-badge">{professionLabel(active)}</strong>
      </p>

      <div role="group" aria-label="Compare with another profession" className="mt-2 flex flex-wrap gap-2">
        {everyday && (
          <button
            type="button"
            aria-pressed={active.id === everyday.id}
            onClick={() => setActiveId(everyday.id)}
            className={pillClass(active.id === everyday.id)}
          >
            Everyday Life
          </button>
        )}
        {JOB_FIELDS.map((field) => (
          <button
            key={field.code}
            type="button"
            aria-pressed={active.job_field_code === field.code}
            disabled={pending !== undefined}
            onClick={() => void selectField(field.code)}
            className={pillClass(active.job_field_code === field.code)}
          >
            {pending === field.code ? `${field.label}…` : field.label}
          </button>
        ))}
      </div>
      {failedField && (
        <p role="alert" className="mt-2 text-sm text-[#9b2c1f]">
          Couldn&apos;t explain this for {fieldLabel(failedField)}. Try again in a moment.
        </p>
      )}

      {active.terms.length > 0 && (
        <SectionBlock title="Key terms">
          <dl>
            {active.terms.map((term, index) => (
              <div key={term.term} className="mb-3">
                <dt className="font-semibold">
                  {term.linked_chapter_slug ? (
                    <a
                      className={`text-[#0f5c52] underline underline-offset-2 ${focusRing}`}
                      href={`/chapters/${encodeURIComponent(term.linked_chapter_slug)}`}
                    >
                      {term.term}
                    </a>
                  ) : (
                    term.term
                  )}
                </dt>
                <dd>{term.definition}</dd>
                <dd>{drawer('term', index)}</dd>
              </div>
            ))}
          </dl>
        </SectionBlock>
      )}

      <SectionBlock title="Step by step, from your work">
        <ol>
          {active.analogy_steps.map((step, index) => (
            <li key={step.step_number} className="mb-5 grid grid-cols-[1.75rem_1fr] gap-x-1">
              <span aria-hidden className="font-semibold text-[#0f5c52]">
                {step.step_number}
              </span>
              <div>
                <p className="flex flex-wrap items-baseline gap-x-2 font-semibold">
                  <span>{step.work_element}</span>
                  <span className="font-normal text-[#35524c]">works like</span>
                  <span className="bg-[#fbe96b] px-1">{step.subject_term}</span>
                </p>
                <p className="mt-1">{step.explanation}</p>
                {drawer('step', index)}
              </div>
            </li>
          ))}
        </ol>
      </SectionBlock>

      <section className="mt-7 rounded-md border-2 border-dashed border-[#b4670f] bg-[#fff6e8] p-4">
        <h3 className="mb-2 text-base font-semibold text-[#6e3f08]">Where this comparison falls short</h3>
        <ul>
          {active.analogy_limits.map((limit, index) => (
            <li key={limit.analogy_limit} className="mb-3 last:mb-0">
              <p className="font-semibold">{limit.analogy_limit}</p>
              <p>{limit.accurate_explanation}</p>
              {drawer('limit', index)}
            </li>
          ))}
        </ul>
      </section>

      {active.prerequisites.length > 0 && (
        <SectionBlock title="Master these first">
          <ul>
            {active.prerequisites.map((link) => (
              <li key={link.chapter_id} className="mb-2">
                <a
                  className={`text-[#0f5c52] underline underline-offset-2 ${focusRing}`}
                  href={`/chapters/${encodeURIComponent(link.slug)}`}
                >
                  {link.title}
                </a>
                <span className="block text-sm text-[#35524c]">{link.reason}</span>
              </li>
            ))}
          </ul>
        </SectionBlock>
      )}

      <FeedbackControls
        key={active.id}
        explanation={active}
        onFeedback={onFeedback}
        onSaved={saveFeedback}
      />
    </aside>
  );
}
