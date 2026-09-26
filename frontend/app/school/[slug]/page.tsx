"use client";
import {useEffect,useState} from "react";
import {useParams} from "next/navigation";
import Link from "next/link";

export default function SchoolHome(){
 const {slug}=useParams<{slug:string}>();
 const [d,setD]=useState<any>();
 const [e,setE]=useState("");
 useEffect(()=>{fetch("/backend/public/schools/"+encodeURIComponent(String(slug))+"/content").then(async r=>{const x=await r.json();if(!r.ok)throw Error(x?.error||"School not found");setD(x)}).catch(x=>setE(x.message))},[slug]);
 if(e)return <main className="content"><div className="error banner">{e}</div></main>;
 if(!d)return <main className="content"><div className="panel">Loading school website…</div></main>;
 const s=d.school,posts=d.featured_blog||[],albums=d.featured_gallery||[],ads=d.advertisements||[];
 return <main style={{minHeight:"100vh",background:"#f7f8fb"}}>
  <header style={{background:"#fff",borderBottom:"1px solid #e5e7eb"}}>
   <div style={{maxWidth:1180,margin:"0 auto",padding:"16px 24px",display:"flex",alignItems:"center",justifyContent:"space-between",gap:20}}>
    <Link href={"/school/"+s.slug} style={{display:"flex",alignItems:"center",gap:12,textDecoration:"none",color:"inherit"}}>
     {s.logo_url ? <img src={s.logo_url} alt={s.name+" logo"} style={{width:48,height:48,objectFit:"contain"}}/> : <span>🏫</span>}
     <strong>{s.name}</strong>
    </Link>
    <nav style={{display:"flex",gap:16}}>
     <Link href={"/school/"+s.slug+"/blog"}>Blog</Link>
     <Link href={"/school/"+s.slug+"/gallery"}>Gallery</Link>
     <Link href={"/school/"+s.slug+"/login"}>Login</Link>
    </nav>
   </div>
  </header>
  <section style={{background:"linear-gradient(135deg,#0d2175,#173aa5)",color:"#fff"}}>
   <div style={{maxWidth:1180,margin:"0 auto",padding:"80px 24px"}}>
    <p>WELCOME TO</p><h1 style={{fontSize:"clamp(2.2rem,5vw,4.5rem)",margin:"8px 0"}}>{s.name}</h1>
    <p style={{fontSize:20,maxWidth:700}}>{s.description||"Discover our school community, news, events and achievements."}</p>
   </div>
  </section>
  {ads.length>0&&<section style={{maxWidth:1180,margin:"28px auto 0",padding:"0 24px"}}>
   <div style={{background:"#fff",borderRadius:20,overflow:"hidden",border:"1px solid #e5e7eb",boxShadow:"0 12px 30px rgba(15,23,42,.08)"}}>
    <div style={{padding:"12px 20px",fontSize:12,fontWeight:800,letterSpacing:1.5,color:"#0d2175",textTransform:"uppercase"}}>Advertisement</div>
    {ads.map((a:any)=><div key={a.id} style={{display:"grid",gridTemplateColumns:"minmax(0,1.35fr) minmax(280px,1fr)",alignItems:"stretch"}}>
      <img src={a.image_url} alt={a.title} style={{width:"100%",height:300,objectFit:"cover"}}/>
      <div style={{padding:"34px 30px",display:"flex",flexDirection:"column",justifyContent:"center"}}>
       <h2 style={{fontSize:"clamp(1.7rem,3vw,2.5rem)",margin:"0 0 12px"}}>{a.title}</h2>
       {a.description&&<p style={{fontSize:17,lineHeight:1.6,color:"#475569"}}>{a.description}</p>}
       {a.target_url&&<a href={a.target_url} target="_blank" rel="noreferrer" style={{display:"inline-block",marginTop:18,padding:"12px 20px",borderRadius:10,background:"#0d2175",color:"#fff",textDecoration:"none",fontWeight:700,width:"fit-content"}}>{a.button_text||"Learn More"} →</a>}
      </div>
    </div>)}
   </div>
  </section>}
  <div style={{maxWidth:1180,margin:"0 auto",padding:"48px 24px"}}>
   <section><div style={{display:"flex",justifyContent:"space-between"}}><h2>Latest News</h2><Link href={"/school/"+s.slug+"/blog"}>View all</Link></div>
    <div style={{display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(240px,1fr))",gap:20}}>
     {posts.map((p:any)=><article key={p.id} style={{background:"#fff",borderRadius:16,overflow:"hidden",border:"1px solid #e5e7eb"}}>
      {p.featured_image_url&&<img src={p.featured_image_url} alt={p.title} style={{width:"100%",height:180,objectFit:"cover"}}/>}
      <div style={{padding:20}}><h3>{p.title}</h3><p>{p.excerpt||String(p.content).slice(0,140)}</p><Link href={"/school/"+s.slug+"/blog/"+p.slug}>Read more →</Link></div>
     </article>)}
    </div>
    {!posts.length&&<div className="panel">No published news yet.</div>}
   </section>
   <section style={{marginTop:52}}><div style={{display:"flex",justifyContent:"space-between"}}><h2>School Gallery</h2><Link href={"/school/"+s.slug+"/gallery"}>View gallery</Link></div>
    <div style={{display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(220px,1fr))",gap:20}}>
     {albums.map((a:any)=><Link key={a.id} href={"/school/"+s.slug+"/gallery/"+a.slug} style={{textDecoration:"none",color:"inherit",background:"#fff",borderRadius:16,overflow:"hidden",border:"1px solid #e5e7eb"}}>
      {a.cover_image_url?<img src={a.cover_image_url} alt={a.title} style={{width:"100%",height:180,objectFit:"cover"}}/>:<div style={{height:180,display:"grid",placeItems:"center",background:"#eef2ff"}}>📷</div>}
      <div style={{padding:18}}><strong>{a.title}</strong><p>{a.description}</p></div>
     </Link>)}
    </div>
    {!albums.length&&<div className="panel">No published gallery albums yet.</div>}
   </section>
   <footer style={{marginTop:64,padding:"28px 0",borderTop:"1px solid #e5e7eb"}}><strong>{s.name}</strong><span style={{float:"right"}}>Built with Stonez Digital</span></footer>
  </div>
 </main>;
}