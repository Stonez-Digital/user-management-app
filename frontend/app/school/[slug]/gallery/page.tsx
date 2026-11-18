"use client";
import {useEffect,useState} from "react";
import Link from "next/link";
import {useParams} from "next/navigation";
export default function Gallery(){
 const{slug}=useParams<{slug:string}>();const[d,setD]=useState<any>();const[e,setE]=useState("");
 useEffect(()=>{fetch("/backend/public/schools/"+encodeURIComponent(String(slug))+"/gallery").then(async r=>{const x=await r.json();if(!r.ok)throw Error(x?.error||"Unable to load gallery");setD(x)}).catch(x=>setE(x.message))},[slug]);
 if(e)return <main className="content"><div className="error banner">{e}</div></main>;
 if(!d)return <main className="content"><div className="panel">Loading gallery…</div></main>;
 return <main className="content" style={{maxWidth:1180,margin:"0 auto"}}><Link href={"/school/"+slug}>← School home</Link><header className="topbar"><div><p className="eyebrow">SCHOOL GALLERY</p><h1>Gallery</h1></div></header><div style={{display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(260px,1fr))",gap:22}}>{(d.albums||[]).map((a:any)=><Link key={a.id} href={"/school/"+slug+"/gallery/"+a.slug} className="panel" style={{textDecoration:"none",color:"inherit"}}>{a.cover_image_url&&<img src={a.cover_image_url} alt={a.title} style={{width:"100%",height:210,objectFit:"cover",borderRadius:12}}/>}<h2>{a.title}</h2><p>{a.description}</p></Link>)}</div></main>;
}