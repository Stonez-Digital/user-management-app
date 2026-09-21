"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { landingPathForRole, normalizeAuthUser } from "../lib/auth-session";

async function readResponse(response: Response) {
 const text = await response.text();
 if (!text) return {};
 try { return JSON.parse(text); }
 catch { return { error: response.ok ? "Invalid API response" : `API returned HTTP ${response.status}: ${text.slice(0,160)}` }; }
}

export default function LoginPage() {
 const router=useRouter();
 const [email,setEmail]=useState("");
 const [password,setPassword]=useState("");
 const [schoolCode,setSchoolCode]=useState(()=>typeof window==="undefined"?"":localStorage.getItem("school_code")||"");
 const [showSchool,setShowSchool]=useState(false);
 const [error,setError]=useState("");
 const [loading,setLoading]=useState(false);

 async function login(e:React.FormEvent){
  e.preventDefault(); setLoading(true); setError("");
  try {
   const r=await fetch("/backend/auth/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({email,password,...(schoolCode.trim()?{school_code:schoolCode.trim()}: {})})});
   const d=await readResponse(r);
   if(!r.ok) throw Error(d?.error?.message||d?.error||"Login failed");
   if(!d.access_token||!d.refresh_token) throw Error("Login response is missing authentication tokens");
   localStorage.setItem("access_token",d.access_token); localStorage.setItem("refresh_token",d.refresh_token);
   if(schoolCode.trim()) localStorage.setItem("school_code",schoolCode.trim().toUpperCase());
   const me=await fetch("/backend/me",{headers:{Authorization:"Bearer "+d.access_token}});
   const payload=await readResponse(me);
   const user=normalizeAuthUser(payload);
   if(!me.ok || !user){
    localStorage.removeItem("access_token"); localStorage.removeItem("refresh_token");
    throw Error((payload as any)?.error?.message||(payload as any)?.error||`Unable to load your account after sign-in (HTTP ${me.status})`);
   }
   router.push(landingPathForRole(user.role));
  } catch(e) { setError(e instanceof Error?e.message:"Login failed"); } finally { setLoading(false); }
 }
 return <main className="auth-shell">
  <section className="auth-card auth-card-wide">
   <div className="auth-brand"><img className="auth-logo" src="/stonez-digital-logo.svg" alt="Stonez Digital" /><div><p className="eyebrow">STONEZ DIGITAL</p><span className="tenant-badge">Multi-school platform</span></div></div>
   <h1>School Management</h1>
   <p className="muted">Securely sign in to your school workspace.</p>
   <div className="tenant-note"><strong>One platform. Separate schools.</strong><span>Your account can only access the school it belongs to.</span></div>
   <form onSubmit={login} className="form">
    <label>School code <button type="button" className="link-button" onClick={()=>setShowSchool(v=>!v)}>{showSchool?"Hide":"I have a school code"}</button>
     {showSchool&&<input value={schoolCode} onChange={e=>setSchoolCode(e.target.value.toUpperCase())} placeholder="e.g. SCHOOL-A" autoCapitalize="characters"/>}
    </label>
    <label>Email<input type="email" value={email} onChange={e=>setEmail(e.target.value)} placeholder="admin@school.com" autoComplete="username" required/></label>
    <label>Password<input type="password" value={password} onChange={e=>setPassword(e.target.value)} placeholder="••••••••" autoComplete="current-password" required/></label>
    {error&&<div className="error">{error}</div>}
    <button disabled={loading}>{loading?"Signing in...":"Sign in to school"}</button>
   </form>
   <div className="auth-footer"><span>Need a new school workspace?</span><Link href="/school-signup">Register your school</Link></div>
   <p className="hint">Protected multi-tenant workspace · Connected to Stonez Digital School API</p>
  </section>
 </main>;
}