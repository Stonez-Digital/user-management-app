"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";

type Option = { id: string; label: string };
type Question = { id: string; prompt: string; help?: string; options: Option[]; multiple: boolean };
type Assessment = { id: string; version: string; title: string; description: string; questions: Question[] };
type Match = { career: { id: string; title: string; description: string; relevant_subjects: string[]; skills_to_explore: string[]; exploration_activity: string }; matched_interests: string[]; shared_tags: number; explanation: string };
type Attempt = { id: string; version: string; status: string; responses: Record<string, string[]>; results?: Match[]; submitted_at?: string };
async function api(path: string, method = "GET", body?: unknown) {
  const token = localStorage.getItem("access_token");
  const response = await fetch("/backend" + path, { method, headers: { Authorization: "Bearer " + token, ...(body ? { "Content-Type": "application/json" } : {}) }, ...(body ? { body: JSON.stringify(body) } : {}) });
  const data = await response.json().catch(() => ({}));
  if (response.status === 401) throw new Error("Your session has expired. Sign in again to continue.");
  if (!response.ok) throw new Error(data?.error?.message || data?.error || "CareerPath could not complete that request.");
  return data;
}

export default function CareerPathPage() {
  const [assessment, setAssessment] = useState<Assessment | null>(null);
  const [attempt, setAttempt] = useState<Attempt | null>(null);
  const [responses, setResponses] = useState<Record<string, string[]>>({});
  const [step, setStep] = useState(0);
  const [reviewing, setReviewing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const draft = await api("/student/careerpath/attempts", "POST") as Attempt;
        const form = await api("/student/careerpath/assessment?version=" + encodeURIComponent(draft.version)) as Assessment;
        setAssessment(form);
        setAttempt(draft);
        setResponses(draft.responses || {});
      } catch (e) { setError(e instanceof Error ? e.message : "Unable to load CareerPath."); }
      finally { setLoading(false); }
    })();
  }, []);

  const question = assessment?.questions[step];
  const dimensionLabels: Record<string, string> = { interests: "Interests", personality_strengths: "Personality and strengths", work_style: "Work style", workplace_preferences: "Workplace preferences", subjects: "Subjects and skills", education_experience: "Education and experience", goals: "Next steps and goals" };
  const currentDimension = question ? dimensionLabels[question.id] || "Career exploration" : "";
  const answered = useMemo(() => !!assessment && assessment.questions.every(q => (responses[q.id] || []).length > 0), [assessment, responses]);
  function toggle(option: string) {
    if (!question || attempt?.status === "submitted") return;
    setResponses(old => {
      const selected = old[question.id] || [];
      const next = question.multiple
        ? (selected.includes(option) ? selected.filter(x => x !== option) : [...selected, option])
        : [option];
      return { ...old, [question.id]: next };
    });
    setNotice("");
  }
  async function save() {
    if (!attempt) return;
    setSaving(true); setError(""); setNotice("");
    try {
      const updated = await api("/student/careerpath/attempts/" + attempt.id + "/responses", "PUT", { responses });
      setAttempt(updated); setResponses(updated.responses || responses); setNotice("Your progress has been saved. You can return to this assessment later.");
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to save your progress."); }
    finally { setSaving(false); }
  }
  async function submit() {
    if (!attempt || !answered) return;
    setSaving(true); setError(""); setNotice("");
    try {
      await api("/student/careerpath/attempts/" + attempt.id + "/responses", "PUT", { responses });
      const result = await api("/student/careerpath/attempts/" + attempt.id + "/submit", "POST");
      setAttempt(result); setResponses(result.responses || responses); setNotice("Your exploration results are ready.");
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to submit your assessment."); }
    finally { setSaving(false); }
  }

  if (loading) return <main className="content standalone"><p role="status">Loading CareerPath…</p></main>;
  if (error && !assessment) return <main className="content standalone"><Link className="back" href="/dashboard/student">← Student dashboard</Link><div className="error banner" role="alert">{error}</div></main>;
  if (!assessment || !attempt) return <main className="content standalone"><div className="panel"><h1>CareerPath is unavailable</h1><p>Please try again shortly.</p></div></main>;

  return <main className="content standalone">
    <Link className="back" href="/dashboard/student">← Student dashboard</Link>
    <header className="topbar"><div><p className="eyebrow">STONEZ DIGITAL · CAREER EXPLORATION</p><h1>{assessment.title}</h1><p className="muted">{assessment.description}</p></div></header>
    {attempt.version === "careerpath-v1" && attempt.status !== "submitted" && <div className="panel" role="status" style={{ marginBottom: 16 }}><strong>Continuing an earlier assessment</strong><p className="muted">Your saved answers belong to an earlier version of CareerPath. You can continue and submit this assessment without losing your progress.</p></div>}
    <div className="panel" style={{ marginBottom: 16 }}><strong>What you will explore</strong><p className="muted">Interests · Strengths · Work settings · Subjects and skills · Experience · Next steps</p><p className="muted">You can select more than one answer on each question. Your choices help start a conversation about possibilities; they do not define your personality or limit your future.</p></div>
    <div className="panel">
      <div className="panel-head"><div><h2>{attempt.status === "submitted" ? "Your career ideas" : "Discover what interests you"}</h2><p>{attempt.status === "submitted" ? "Explore these possibilities at your own pace. You can change your interests over time." : "Choose the answers that feel right to you. You can save and resume whenever you need."}</p></div><span className="badge">{attempt.status === "submitted" ? "Completed" : "In progress"}</span></div>
      {error && <div className="error banner" role="alert">{error}</div>}
      {notice && <p role="status" className="muted">{notice}</p>}
      {attempt.status !== "submitted" && question && !reviewing && <>
        <div aria-label={"Question " + (step + 1) + " of " + assessment.questions.length} className="muted">Question {step + 1} of {assessment.questions.length}</div>
        <div style={{ height: 8, background: "var(--border, #e5e7eb)", borderRadius: 999, margin: "12px 0 24px" }}><div style={{ height: "100%", width: ((step + 1) / assessment.questions.length * 100) + "%", background: "var(--primary, #0D2175)", borderRadius: 999 }} /></div>
        <p className="eyebrow">{currentDimension}</p><h2>{question.prompt}</h2>{question.help && <p className="muted">{question.help}</p>}
        <fieldset style={{ border: 0, padding: 0, margin: "16px 0" }}><legend className="muted">{question.multiple ? "Select all that interest you." : "Choose one answer."}</legend>
          <div className="module-grid">{question.options.map(option => {
            const selected = (responses[question.id] || []).includes(option.id);
            return <button type="button" key={option.id} aria-pressed={selected} onClick={() => toggle(option.id)} className="module" style={{ textAlign: "left", cursor: "pointer", border: selected ? "2px solid var(--primary, #0D2175)" : undefined, background: selected ? "var(--surface-muted, #f3f5ff)" : undefined }}><span aria-hidden="true">{selected ? "✓" : "○"}</span><span>{option.label}</span></button>;
          })}</div>
        </fieldset>
        <div className="panel-head" style={{ flexWrap: "wrap", gap: 8 }}>
          <button type="button" className="back" disabled={step === 0 || saving} onClick={() => setStep(s => Math.max(0, s - 1))}>← Back</button>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button type="button" disabled={saving} onClick={save}>{saving ? "Saving…" : "Save progress"}</button>
            {step < assessment.questions.length - 1
              ? <button type="button" disabled={!(responses[question.id] || []).length} onClick={() => setStep(s => Math.min(assessment.questions.length - 1, s + 1))}>Next →</button>
              : <button type="button" disabled={!answered || saving} onClick={() => setReviewing(true)}>Review answers →</button>}
          </div>
        </div>
        <p className="muted">Your answers are private to your account in this first release. CareerPath offers ideas for exploration, not a diagnosis or a decision about your future.</p>
      </>}
      {attempt.status !== "submitted" && reviewing && <section aria-labelledby="careerpath-review-title"><h2 id="careerpath-review-title">Review your answers</h2><p className="muted">Check your selections before submitting. You can go back and change anything.</p>{assessment.questions.map((q, index) => <div className="module" key={q.id} style={{ alignItems: "flex-start" }}><div><strong>{index + 1}. {q.prompt}</strong><p>{(responses[q.id] || []).map(id => q.options.find(o => o.id === id)?.label || id).join(", ") || "Not answered"}</p></div><button type="button" onClick={() => { setStep(index); setReviewing(false); }}>Edit</button></div>)}<div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginTop: 16 }}><button type="button" disabled={saving} onClick={() => setReviewing(false)}>← Back to questions</button><button type="button" disabled={!answered || saving} onClick={submit}>{saving ? "Submitting…" : "Confirm and submit"}</button></div></section>}
      {attempt.status === "submitted" && <><div className="module-grid">{(attempt.results || []).map(match => <article className="module" key={match.career.id} style={{ alignItems: "flex-start" }}><div><h3>{match.career.title}</h3><p>{match.career.description}</p><p><strong>Why it appeared</strong><br />{match.explanation}</p><p><strong>Subjects to explore</strong><br />{match.career.relevant_subjects.join(", ")}</p><p><strong>Skills to develop</strong><br />{match.career.skills_to_explore.join(", ")}</p><p><strong>Try this activity</strong><br />{match.career.exploration_activity}</p></div></article>)}</div><p className="muted">These suggestions use transparent overlap between your selected interests and a small starter catalogue. They are not scientifically validated predictions, and they do not determine admissions, placement, or future opportunities.</p></>}
    </div>
  </main>;
}
