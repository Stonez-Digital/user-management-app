"use client";
import {useEffect,useState} from "react";
import Link from "next/link";
import {useParams} from "next/navigation";
export default function Blog(){
 const {slug}=useParams<{slug:string}>(); const [d,setD]=useState<any>(); const [e,setE]=useState("");
 useEffect(()=>{fetch("/backend/public/schools/"+encodeURIComponent(String(slug))+"/blog").then(async r=>{const x=await r.json();if(!r.ok)throw Error(x?.error||"Unable to load blog");setD(x)}).catch(x=>setE(x.message))},[slug]);
 if(e)return <main className="content"><div className="error banner">{e}</div></main>;
 if(!d)return <main className="content"><div className="panel">Loading blog…</div></main>;
 return <main className="content" style={{maxWidth:1180,margin:"0 auto"}}><Link href={"/school/"+slug}>← {d.school.name}</Link><header className="topbar"><div><p className="eyebrow">SCHOOL BLOG</p><h1>{d.school.name}</h1><p className="muted">News, announcements and school stories.</p></div></header><div style={{display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(280px,1fr))",gap:22}}>{(d.posts||[]).map((p:any)=><article className="panel" key={p.id}>{p.featured_image_url&&<img src={p.featured_image_url} alt={p.title} style={{width:"100%",height:190,objectFit:"cover",borderRadius:12}}/>}<h2>{p.title}</h2><small>{p.published_at?new Date(p.published_at).toLocaleDateString():""}</small><p>{p.excerpt||p.content.slice(0,180)}</p><Link href={"/school/"+slug+"/blog/"+p.slug}>Read article →</Link></article>)}</div></main>;
}