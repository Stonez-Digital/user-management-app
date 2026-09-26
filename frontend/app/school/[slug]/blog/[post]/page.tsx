"use client";
import {useEffect,useState} from "react";
import Link from "next/link";
import {useParams} from "next/navigation";
export default function Post(){
 const p=useParams<{slug:string;post:string}>();const[d,setD]=useState<any>();const[e,setE]=useState("");
 useEffect(()=>{fetch("/backend/public/schools/"+encodeURIComponent(String(p.slug))+"/blog/"+encodeURIComponent(String(p.post))).then(async r=>{const x=await r.json();if(!r.ok)throw Error(x?.error||"Article not found");setD(x)}).catch(x=>setE(x.message))},[p.slug,p.post]);
 if(e)return <main className="content"><div className="error banner">{e}</div></main>;
 if(!d)return <main className="content"><div className="panel">Loading article…</div></main>;
 return <main className="content" style={{maxWidth:900,margin:"0 auto"}}><Link href={"/school/"+p.slug+"/blog"}>← Back to blog</Link><article className="panel" style={{marginTop:24}}>{d.featured_image_url&&<img src={d.featured_image_url} alt={d.title} style={{width:"100%",maxHeight:520,objectFit:"cover",borderRadius:12}}/>}<p className="eyebrow">SCHOOL NEWS</p><h1>{d.title}</h1><small>{d.published_at?new Date(d.published_at).toLocaleDateString():""}</small><p style={{whiteSpace:"pre-wrap",lineHeight:1.8,marginTop:24}}>{d.content}</p></article></main>;
}