"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";

async function readResponse(response: Response) {
 const text = await response.text();
 if (!text) return {};
 try { return JSON.parse(text); }
 catch { return { error: response.ok ? "Invalid API response" : `API returned HTTP ${response.status}: ${text.slice(0,160)}` }; }
}

export default function LoginPage() {
 const router=useRouter(); const [email,setEmail]=useState(""); const [password,setPassword]=useState(""); const [error,setError]=useState(""); const [loading,setLoading]=useState(false);
 async function login(e:React.FormEvent){
  e.preventDefault(); setLoading(true); setError("");
  try {
   const r=await fetch("/backend/auth/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({email,password})});
   const d=await readResponse(r);
   if(!r.ok) throw Error(d?.error?.message||d?.error||"Login failed");
   if(!d.access_token||!d.refresh_token) throw Error("Login response is missing authentication tokens");
   localStorage.setItem("access_token",d.access_token); localStorage.setItem("refresh_token",d.refresh_token); router.push("/dashboard");
  } catch(e) { setError(e instanceof Error?e.message:"Login failed"); } finally { setLoading(false); }
 }
 return <main className="auth-shell"><section className="auth-card"><div className="brand-mark">S</div><p className="eyebrow">STONEZ DIGITAL</p><h1>School Management</h1><p className="muted">Sign in to manage your school operations.</p><form onSubmit={login} className="form"><label>Email<input type="email" value={email} onChange={e=>setEmail(e.target.value)} placeholder="admin@school.com" required/></label><label>Password<input type="password" value={password} onChange={e=>setPassword(e.target.value)} placeholder="••••••••" required/></label>{error&&<div className="error">{error}</div>}<button disabled={loading}>{loading?"Signing in...":"Sign in"}</button></form><p className="hint">Go API: localhost:8080</p></section></main>;
}