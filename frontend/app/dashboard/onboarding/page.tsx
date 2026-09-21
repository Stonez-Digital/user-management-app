"use client";

import { useEffect, useState } from "react";
import Link from "next/link";

type User = { id: string; name: string; email: string; role: string; active: boolean };
type Student = { id: string; user_id: string; admission_number: string; user?: User };

async function api(path: string, options: RequestInit = {}) {
  const token = localStorage.getItem("access_token");
  const r = await fetch("/backend" + path, {
    ...options,
    headers: { "Content-Type": "application/json", Authorization: "Bearer " + token, ...(options.headers || {}) },
  });
  const text = await r.text();
  let d: any = {};
  try { d = text ? JSON.parse(text) : {}; } catch { d = { error: text }; }
  if (r.status === 401) throw new Error("Session expired");
  if (!r.ok) throw new Error(d?.error?.message || d?.error || "Request failed");
  return d;
}
const list = (d: any, key: string) => Array.isArray(d) ? d : d?.[key] || [];

export default function OnboardingPage() {
  const [role, setRole] = useState("teacher");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [admission, setAdmission] = useState("");
  const [relationship, setRelationship] = useState("parent");
  const [studentId, setStudentId] = useState("");
  const [students, setStudents] = useState<Student[]>([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function loadStudents() {
    try { setStudents(list(await api("/admin/students"), "students")); } catch {}
  }
  useEffect(() => { loadStudents(); }, []);

  function reset() {
    setName(""); setEmail(""); setPassword(""); setAdmission(""); setMessage(""); setError("");
  }

  async function create() {
    if (!name.trim() || !email.trim() || password.length < 8) {
      setError("Name, email and a password of at least 8 characters are required.");
      return;
    }
    if (role === "student" && !admission.trim()) {
      setError("Admission number is required before creating a student account.");
      return;
    }
    setBusy(true); setError(""); setMessage("");
    try {
      const user = await api("/admin/users", {
        method: "POST",
        body: JSON.stringify({ name: name.trim(), email: email.trim(), password, role }),
      });
      const created = user?.id ? user : user?.user;
      if (role === "student") {
        if (!admission.trim()) throw new Error("Student account created, but admission number is required to create the student profile.");
        const student = await api("/admin/students", {
          method: "POST",
          body: JSON.stringify({ user_id: created.id, admission_number: admission.trim(), enrollment_status: "active" }),
        });
        setMessage("Student account and profile created. Continue to Enrollment to place the student in a class/section.");
        await loadStudents();
      } else if (role === "parent" && studentId) {
        await api("/admin/guardian-links", {
          method: "POST",
          body: JSON.stringify({ guardian_user_id: created.id, student_id: studentId, relationship, primary: true }),
        });
        setMessage("Parent account and student link created.");
      } else {
        setMessage(role === "teacher" ? "Teacher account created. Continue to Teacher Assignments to allocate teaching responsibilities." : "Account created successfully.");
      }
      reset();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to complete onboarding");
    } finally { setBusy(false); }
  }

  return <div className="content standalone">
    <header className="topbar">
      <div>
        <Link className="back" href="/dashboard">← Dashboard</Link>
        <p className="eyebrow">SCHOOL ONBOARDING</p>
        <h1>Onboard people</h1>
        <p className="muted">Create school-scoped teacher, student and parent accounts without direct database access.</p>
      </div>
    </header>

    {error && <div className="error banner">{error}</div>}
    {message && <div className="panel"><strong>{message}</strong></div>}

    <section className="panel">
      <div className="panel-head">
        <div><h2>New school account</h2><p>Accounts are created inside the currently authenticated school.</p></div>
      </div>
      <div className="form-grid">
        <label>Role
          <select value={role} onChange={e => { setRole(e.target.value); setMessage(""); }}>
            <option value="teacher">Teacher</option>
            <option value="student">Student</option>
            <option value="parent">Parent / Guardian</option>
            <option value="accountant">Accountant</option>
            <option value="staff">Staff</option>
          </select>
        </label>
        <label>Full name<input value={name} onChange={e => setName(e.target.value)} placeholder="Full name"/></label>
        <label>Email<input type="email" value={email} onChange={e => setEmail(e.target.value)} placeholder="person@example.com"/></label>
        <label>Temporary password<input type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder="Minimum 8 characters"/></label>
        {role === "student" && <label>Admission number<input value={admission} onChange={e => setAdmission(e.target.value)} placeholder="BED-001"/></label>}
        {role === "parent" && <label>Link to student
          <select value={studentId} onChange={e => setStudentId(e.target.value)}>
            <option value="">Create account only</option>
            {students.map(s => <option key={s.id} value={s.id}>{s.user?.name || s.admission_number} — {s.admission_number}</option>)}
          </select>
        </label>}
        {role === "parent" && studentId && <label>Relationship<input value={relationship} onChange={e => setRelationship(e.target.value)} placeholder="parent"/></label>}
        <div className="form-action"><button disabled={busy} onClick={create}>{busy ? "Creating..." : "Create account"}</button><button className="ghost" onClick={reset}>Clear</button></div>
      </div>
    </section>

    <section className="panel">
      <h2>Next steps</h2>
      <p className="muted">After creating accounts, complete the operational records using the existing workflows.</p>
      <div className="actions">
        <Link className="ghost" href="/dashboard/students">Student profiles</Link>
        <Link className="ghost" href="/dashboard/enrollments">Enrollment</Link>
        <Link className="ghost" href="/dashboard/teacher-assignments">Teacher assignments</Link>
        <Link className="ghost" href="/dashboard/users">Users & Roles</Link>
      </div>
    </section>
  </div>;
}
