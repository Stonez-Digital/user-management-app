"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";

type User = { id: string; name: string; email: string; role: string; active: boolean };
type Session = { id: string; name: string; start_date: string; end_date: string; status: string };
type Term = { id: string; name: string; academic_session_id: string; status: string };
type SchoolClass = { id: string; name: string; level: number; sections?: Section[] };
type Section = { id: string; class_id: string; name: string };
type Subject = { id: string; code: string; name: string; active: boolean };
type Assignment = {
  id: string; teacher_id: string; subject_id: string; academic_session_id: string; term_id: string;
  class_id: string; section_id?: string | null; allocation_type: string; active: boolean;
  teacher?: User; subject?: Subject;
};

async function api(path: string, options: RequestInit = {}) {
  const token = localStorage.getItem("access_token");
  const response = await fetch("/backend" + path, {
    ...options,
    headers: { "Content-Type": "application/json", Authorization: "Bearer " + token, ...(options.headers || {}) },
  });
  if (response.status === 401) throw new Error("Session expired");
  const text = await response.text();
  let data: any = {};
  try { data = text ? JSON.parse(text) : {}; } catch { data = { error: text }; }
  if (!response.ok) throw new Error(data?.error?.message || data?.error || "Request failed");
  return data;
}

const list = (data: any, key: string) => Array.isArray(data) ? data : data?.[key] || [];

export default function TeacherAssignmentsPage() {
  const [teachers, setTeachers] = useState<User[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [terms, setTerms] = useState<Term[]>([]);
  const [classes, setClasses] = useState<SchoolClass[]>([]);
  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [assignments, setAssignments] = useState<Assignment[]>([]);
  const [teacherId, setTeacherId] = useState("");
  const [sessionId, setSessionId] = useState("");
  const [termId, setTermId] = useState("");
  const [classId, setClassId] = useState("");
  const [sectionId, setSectionId] = useState("");
  const [subjectId, setSubjectId] = useState("");
  const [allocationType, setAllocationType] = useState("subject_teacher");
  const [coverageView, setCoverageView] = useState("teacher");
  const [query, setQuery] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [coverage, setCoverage] = useState<Assignment[]>([]);

  const sections = useMemo(() => classes.find(c => c.id === classId)?.sections || [], [classes]);
  const maps = useMemo(() => ({
    teachers: Object.fromEntries(teachers.map(x => [x.id, x.name || x.email])),
    sessions: Object.fromEntries(sessions.map(x => [x.id, x.name])),
    terms: Object.fromEntries(terms.map(x => [x.id, x.name])),
    classes: Object.fromEntries(classes.map(x => [x.id, x.name])),
    sections: Object.fromEntries(classes.flatMap(c => (c.sections || []).map(s => [s.id, s.name]))),
    subjects: Object.fromEntries(subjects.map(x => [x.id, x.name])),
  }), [teachers, sessions, terms, classes, subjects]);

  async function load() {
    try {
      const [u, s, c, sub, a] = await Promise.all([
        api("/admin/users"), api("/admin/academic-sessions"), api("/admin/classes"),
        api("/admin/subjects"), api("/admin/teacher-assignments"),
      ]);
      const users = list(u, "users");
      setTeachers(users.filter((x: User) => x.role === "teacher" && x.active));
      const nextSessions = list(s, "sessions");
      setSessions(nextSessions);
      setClasses(list(c, "classes"));
      setSubjects(list(sub, "subjects"));
      const nextAssignments = list(a, "assignments");
      setAssignments(nextAssignments);
      const coverageData = await api("/admin/teacher-assignments/coverage");
      setCoverage(list(coverageData, "assignments"));
      if (!sessionId && nextSessions.length) setSessionId(nextSessions.find((x: Session) => x.status === "active")?.id || nextSessions[0].id);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to load assignment data");
    }
  }

  async function loadTerms(id: string) {
    if (!id) { setTerms([]); setTermId(""); return; }
    try {
      const data = await api("/admin/academic-sessions/" + id + "/terms");
      const next = list(data, "terms");
      setTerms(next);
      setTermId(next.find((x: Term) => x.status === "active")?.id || next[0]?.id || "");
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to load terms"); }
  }

  async function loadSections(id: string) {
    if (!id) return;
    try {
      const data = await api("/admin/classes/" + id + "/sections");
      const next = list(data, "sections");
      setClasses(current => current.map(c => c.id === id ? { ...c, sections: next } : c));
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to load sections"); }
  }

  useEffect(() => { load(); }, []);
  useEffect(() => { if (sessionId) loadTerms(sessionId); }, [sessionId]);
  useEffect(() => { if (classId) { setSectionId(""); loadSections(classId); } }, [classId]);

  async function create() {
    if (!teacherId || !subjectId || !sessionId || !termId || !classId) {
      setError("Select a teacher, subject, academic session, term and class.");
      return;
    }
    setBusy(true); setError("");
    try {
      await api("/admin/teacher-assignments", {
        method: "POST",
        body: JSON.stringify({
          teacher_id: teacherId, subject_id: subjectId, allocation_type: allocationType, academic_session_id: sessionId,
          term_id: termId, class_id: classId, section_id: sectionId || null, active: true,
        }),
      });
      setTeacherId(""); setSubjectId(""); setSectionId(""); setAllocationType("subject_teacher");
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to create assignment"); }
    finally { setBusy(false); }
  }

  async function toggle(item: Assignment) {
    setError("");
    try {
      await api("/admin/teacher-assignments/" + item.id, {
        method: "PUT",
        body: JSON.stringify({
          teacher_id: item.teacher_id, subject_id: item.subject_id, allocation_type: item.allocation_type || "subject_teacher", academic_session_id: item.academic_session_id,
          term_id: item.term_id, class_id: item.class_id, section_id: item.section_id || null, active: !item.active,
        }),
      });
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to update assignment"); }
  }

  async function remove(id: string) {
    if (!window.confirm("Delete this teacher assignment?")) return;
    setError("");
    try { await api("/admin/teacher-assignments/" + id, { method: "DELETE" }); await load(); }
    catch (e) { setError(e instanceof Error ? e.message : "Unable to delete assignment"); }
  }

  const filtered = assignments.filter(a => {
    const haystack = [
      maps.teachers[a.teacher_id], maps.subjects[a.subject_id], maps.sessions[a.academic_session_id],
      maps.terms[a.term_id], maps.classes[a.class_id], maps.sections[a.section_id || ""],
    ].join(" ").toLowerCase();
    return haystack.includes(query.toLowerCase());
  });

  return <div className="content standalone">
    <header className="topbar">
      <div><Link className="back" href="/dashboard">← Dashboard</Link><p className="eyebrow">ACADEMIC OPERATIONS</p><h1>Teacher & Class Assignments</h1><p className="muted">Assign teachers to subjects, classes and optional sections for each academic term.</p></div>
    </header>
    {error && <div className="error banner">{error}</div>}

    <section className="panel">
      <div className="panel-head"><div><h2>Create assignment</h2><p>All selected records must belong to the same school.</p></div></div>
      <div className="form-grid">
        <label>Teacher<select value={teacherId} onChange={e => setTeacherId(e.target.value)}><option value="">Select teacher</option>{teachers.map(t => <option key={t.id} value={t.id}>{t.name || t.email}</option>)}</select></label>
        <label>Subject<select value={subjectId} onChange={e => setSubjectId(e.target.value)}><option value="">Select subject</option>{subjects.filter(s => s.active).map(s => <option key={s.id} value={s.id}>{s.code} — {s.name}</option>)}</select></label>
        <label>Allocation type<select value={allocationType} onChange={e => setAllocationType(e.target.value)}><option value="subject_teacher">Subject teacher</option><option value="class_teacher">Class/form teacher</option></select></label>
        <label>Academic session<select value={sessionId} onChange={e => setSessionId(e.target.value)}><option value="">Select session</option>{sessions.map(s => <option key={s.id} value={s.id}>{s.name} ({s.status})</option>)}</select></label>
        <label>Term<select value={termId} onChange={e => setTermId(e.target.value)} disabled={!sessionId}><option value="">Select term</option>{terms.map(t => <option key={t.id} value={t.id}>{t.name} ({t.status})</option>)}</select></label>
        <label>Class<select value={classId} onChange={e => setClassId(e.target.value)}><option value="">Select class</option>{classes.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}</select></label>
        <label>Section (optional)<select value={sectionId} onChange={e => setSectionId(e.target.value)} disabled={!classId}><option value="">Whole class</option>{sections.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}</select></label>
        <div className="form-action"><button disabled={busy} onClick={create}>{busy ? "Assigning..." : "Assign teacher"}</button></div>
      </div>
    </section>

    <section className="panel">
      <div className="panel-head"><div><h2>Current assignments</h2><p>{filtered.length} assignment{filtered.length === 1 ? "" : "s"}</p></div><input className="search-inline" placeholder="Search assignments..." value={query} onChange={e => setQuery(e.target.value)}/></div>
      <div className="table-wrap"><table><thead><tr><th>Teacher</th><th>Type</th><th>Subject</th><th>Session / term</th><th>Class</th><th>Section</th><th>Status</th><th>Actions</th></tr></thead>
        <tbody>{filtered.map(a => <tr key={a.id}><td><strong>{maps.teachers[a.teacher_id] || a.teacher?.name || "Teacher"}</strong></td><td>{a.allocation_type === "class_teacher" ? "Class/form" : "Subject"}</td><td>{maps.subjects[a.subject_id] || a.subject?.name || "—"}</td><td>{maps.sessions[a.academic_session_id] || "—"}<small>{maps.terms[a.term_id] || "—"}</small></td><td>{maps.classes[a.class_id] || "—"}</td><td>{maps.sections[a.section_id || ""] || "Whole class"}</td><td><span className={"pill " + (a.active ? "active" : "withdrawn")}>{a.active ? "active" : "inactive"}</span></td><td><button className="ghost" onClick={() => toggle(a)}>{a.active ? "Deactivate" : "Activate"}</button> <button className="ghost" onClick={() => remove(a.id)}>Delete</button></td></tr>)}</tbody>
      </table>{!filtered.length && <div className="empty">No teacher assignments found.</div>}</div>
    </section>
    <section className="panel">
      <div className="panel-head"><div><h2>Teaching coverage</h2><p>Review active and historical allocations by operational view.</p></div>
        <select value={coverageView} onChange={e => setCoverageView(e.target.value)}><option value="teacher">By teacher</option><option value="class">By class</option><option value="subject">By subject</option><option value="section">By section</option></select>
      </div>
      <div className="table-wrap"><table><thead><tr><th>Group</th><th>Teacher</th><th>Subject</th><th>Type</th><th>Session / term</th><th>Status</th></tr></thead>
      <tbody>{coverage.map(a => {
        const group = coverageView === "teacher" ? (maps.teachers[a.teacher_id] || a.teacher?.name || "Teacher")
          : coverageView === "class" ? (maps.classes[a.class_id] || "Class")
          : coverageView === "subject" ? (maps.subjects[a.subject_id] || a.subject?.name || "Subject")
          : (maps.sections[a.section_id || ""] || "Whole class");
        return <tr key={a.id}><td><strong>{group}</strong></td><td>{maps.teachers[a.teacher_id] || a.teacher?.name || "—"}</td><td>{maps.subjects[a.subject_id] || a.subject?.name || "—"}</td><td>{a.allocation_type === "class_teacher" ? "Class/form" : "Subject"}</td><td>{maps.sessions[a.academic_session_id] || "—"} · {maps.terms[a.term_id] || "—"}</td><td>{a.active ? "active" : "inactive"}</td></tr>;
      })}</tbody></table>{!coverage.length && <div className="empty">No teaching coverage records.</div>}</div>
    </section>

  </div>;
}
