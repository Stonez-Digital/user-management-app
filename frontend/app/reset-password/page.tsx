"use client";

import { useEffect, useState } from "react";
import Link from "next/link";

export default function ResetPasswordPage() {
  const [token, setToken] = useState("");
  useEffect(() => {
    setToken(new URLSearchParams(window.location.hash.replace(/^#/, "")).get("token") || "");
  }, []);
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setMessage("");
    if (!token) { setError("This reset link is missing its token. Request a new link."); return; }
    if (password !== confirmPassword) { setError("The passwords do not match."); return; }
    setLoading(true);
    try {
      const response = await fetch("/backend/auth/reset-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token, new_password: password }),
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(payload?.error?.message || payload?.error || "This reset link is invalid or has expired. Request a new link.");
      setMessage("Your password has been changed. Sign in with your new password.");
      setPassword("");
      setConfirmPassword("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to reset your password. Request a new link.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="auth-shell">
      <section className="auth-card auth-card-wide">
        <div className="auth-brand">
          <img className="auth-logo" src="/stonez-digital-logo.svg" alt="Stonez Digital" />
          <p className="eyebrow">SECURE ACCOUNT RECOVERY</p>
        </div>
        <h1>Set a new password</h1>
        <p className="muted">Choose a password with at least 8 characters. Your existing sessions will be signed out after the reset.</p>
        <form onSubmit={submit} className="form">
          <label>New password
            <input type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="new-password" minLength={8} maxLength={128} required />
          </label>
          <label>Confirm new password
            <input type="password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} autoComplete="new-password" minLength={8} maxLength={128} required />
          </label>
          {error && <div className="error" role="alert">{error}</div>}
          {message && <div className="tenant-note" role="status"><strong>Password updated</strong><span>{message}</span></div>}
          <button disabled={loading || !token}>{loading ? "Updating password..." : "Reset password"}</button>
        </form>
        <div className="auth-footer"><Link href="/forgot-password">Request another reset link</Link><span> · </span><Link href="/login">Back to sign in</Link></div>
      </section>
    </main>
  );
}
