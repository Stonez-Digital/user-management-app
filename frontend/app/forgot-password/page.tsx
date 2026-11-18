"use client";

import { useState } from "react";
import Link from "next/link";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLoading(true);
    setMessage("");
    setError("");
    try {
      const response = await fetch("/backend/auth/forgot-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: email.trim() }),
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(payload?.error?.message || payload?.error || "Unable to process your request. Please try again.");
      setMessage("If an account exists for that email, a password reset link will arrive shortly. Check your inbox and spam folder.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to process your request. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="auth-shell">
      <section className="auth-card auth-card-wide">
        <div className="auth-brand">
          <img className="auth-logo" src="/stonez-digital-logo.svg" alt="Stonez Digital" />
          <p className="eyebrow">ACCOUNT RECOVERY</p>
        </div>
        <h1>Forgot your password?</h1>
        <p className="muted">Enter the email address linked to your school portal account. We’ll send a secure, time-limited reset link if the account exists.</p>
        <form onSubmit={submit} className="form">
          <label>Email address
            <input type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="you@school.com" autoComplete="email" maxLength={255} required />
          </label>
          {error && <div className="error" role="alert">{error}</div>}
          {message && <div className="tenant-note" role="status"><strong>Request received</strong><span>{message}</span></div>}
          <button disabled={loading}>{loading ? "Sending request..." : "Send reset link"}</button>
        </form>
        <div className="auth-footer"><Link href="/login">Back to sign in</Link></div>
      </section>
    </main>
  );
}
