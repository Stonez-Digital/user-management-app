"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";

type User = { id: string; name: string; email: string; role: string; active: boolean };
type Student = {
  id: string;
  user_id: string;
  admission_number: string;
  gender: string;
  enrollment_status: string;
  date_of_birth?: string;
  guardian_name?: string;
  guardian_phone?: string;
  guardian_email?: string;
  user?: User;
};

async function api(path: string, options: RequestInit = {}) {
  const token = localStorage.getItem("access_token");
  const r = await fetch("/backend" + path, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + token,
      ...(options.headers || {}),
    },
  });
  if (r.status === 401) throw new Error("Session expired");
  const text = await r.text();
  let d: any = {};
  try { d = text ? JSON.parse(text) : {}; } catch { d = { error: text }; }
  if (!r.ok) throw new Error(d?.error?.message || d?.error || "Request failed");
  return d;
}

const list = (d: any, key: string) => Array.isArray(d) ? d : d?.[key] || [];

export default function Students() {
  const [items, setItems] = useState<Student[]>([]);
  const [query, setQuery] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState<Student | null>(null);
  const [admissionNumber, setAdmissionNumber] = useState("");
  const [gender, setGender] = useState("");
  const [status, setStatus] = useState("active");
  const [guardianName, setGuardianName] = useState("");
  const [guardianPhone, setGuardianPhone] = useState("");
  const [guardianEmail, setGuardianEmail] = useState("");

  async function load() {
    try {
      const d = await api("/admin/students");
      setItems(list(d, "students"));
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to load students");
    }
  }

  useEffect(() => { load(); }, []);

  const filtered = useMemo(
    () => items.filter((s) =>
      ((s.user?.name || "") + " " + (s.user?.email || "") + " " + s.admission_number)
        .toLowerCase()
        .includes(query.toLowerCase())
    ),
    [items, query],
  );

  function reset() {
    setEditing(null);
    setAdmissionNumber("");
    setGender("");
    setStatus("active");
    setGuardianName("");
    setGuardianPhone("");
    setGuardianEmail("");
  }

  function edit(student: Student) {
    setEditing(student);
    setAdmissionNumber(student.admission_number);
    setGender(student.gender || "");
    setStatus(student.enrollment_status || "active");
    setGuardianName(student.guardian_name || "");
    setGuardianPhone(student.guardian_phone || "");
    setGuardianEmail(student.guardian_email || "");
  }

  async function save() {
    if (!editing) return;
    if (!admissionNumber.trim()) {
      setError("Admission number is required.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const body = {
        admission_number: admissionNumber.trim(),
        gender,
        enrollment_status: status,
        guardian_name: guardianName,
        guardian_phone: guardianPhone,
        guardian_email: guardianEmail,
      };
      await api("/admin/students/" + editing.id, {
        method: "PUT",
        body: JSON.stringify(body),
      });
      reset();
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to save student");
    } finally {
      setBusy(false);
    }
  }

  async function remove(student: Student) {
    if (!window.confirm("Delete this student profile?")) return;
    try {
      await api("/admin/students/" + student.id, { method: "DELETE" });
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to delete student");
    }
  }

  return (
    <div className="content standalone">
      <header className="topbar">
        <div>
          <Link className="back" href="/dashboard">← Dashboard</Link>
          <p className="eyebrow">STUDENT MANAGEMENT</p>
          <h1>Students</h1>
          <p className="muted">
            Manage student profiles and academic enrollment. New student accounts are created through the school onboarding workflow.
          </p>
        </div>
        <Link href="/dashboard/onboarding" className="button">+ Onboard student</Link>
      </header>

      {error && <div className="error banner">{error}</div>}

      {editing && (
        <section className="panel">
          <div className="panel-head">
            <div>
              <h2>Edit student profile</h2>
              <p>Update school-owned student information. Account creation remains in the atomic onboarding workflow.</p>
            </div>
            <button className="ghost" onClick={reset}>Cancel</button>
          </div>
          <div className="form-grid">
            <label>Student account<input value={editing.user?.name || editing.user?.email || "Student"} disabled /></label>
            <label>Admission number<input value={admissionNumber} onChange={e => setAdmissionNumber(e.target.value)} placeholder="STU-001" /></label>
            <label>Gender<input value={gender} onChange={e => setGender(e.target.value)} placeholder="Optional" /></label>
            <label>Status
              <select value={status} onChange={e => setStatus(e.target.value)}>
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
                <option value="graduated">Graduated</option>
                <option value="withdrawn">Withdrawn</option>
              </select>
            </label>
            <label>Guardian name<input value={guardianName} onChange={e => setGuardianName(e.target.value)} /></label>
            <label>Guardian phone<input value={guardianPhone} onChange={e => setGuardianPhone(e.target.value)} /></label>
            <label>Guardian email<input type="email" value={guardianEmail} onChange={e => setGuardianEmail(e.target.value)} /></label>
            <div className="form-action"><button disabled={busy} onClick={save}>{busy ? "Saving..." : "Save changes"}</button></div>
          </div>
        </section>
      )}

      <section className="panel">
        <div className="toolbar">
          <input
            placeholder="Search by name, email or admission number..."
            value={query}
            onChange={e => setQuery(e.target.value)}
          />
          <Link href="/dashboard/enrollments" className="ghost">Manage enrollments →</Link>
          <Link href="/dashboard/onboarding" className="ghost">Onboard people →</Link>
        </div>
        <div className="table-wrap">
          <table>
            <thead><tr><th>Name</th><th>Email</th><th>Admission</th><th>Status</th><th>Actions</th></tr></thead>
            <tbody>
              {filtered.map(s => (
                <tr key={s.id}>
                  <td><strong>{s.user?.name || "—"}</strong></td>
                  <td>{s.user?.email || "—"}</td>
                  <td>{s.admission_number}</td>
                  <td><span className={"pill " + s.enrollment_status}>{s.enrollment_status}</span></td>
                  <td>
                    <button className="ghost" onClick={() => edit(s)}>Edit</button>{" "}
                    <button className="ghost" onClick={() => remove(s)}>Delete</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {!filtered.length && <div className="empty">No students found.</div>}
        </div>
      </section>
    </div>
  );
}
