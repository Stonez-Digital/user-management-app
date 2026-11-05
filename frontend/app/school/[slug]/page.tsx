"use client";

import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";

type School = { id: string; name: string; slug: string; logo_url?: string };
type Post = { id: string; title: string; slug: string; excerpt?: string; content: string; featured_image_url?: string; published_at?: string };
type Album = { id: string; title: string; slug: string; description?: string; cover_image_url?: string };
type Advertisement = { id: string; title: string; description?: string; image_url: string; button_text?: string; target_url?: string };
type HomepageData = { school: School; featured_blog?: Post[]; featured_gallery?: Album[]; advertisements?: Advertisement[] };

const styles = `
.school-site{--ink:#14213d;--muted:#66738b;--blue:#173b8f;--blue-dark:#10265f;--line:#e8edf5;min-height:100vh;background:#f7f9fc;color:var(--ink);font-family:Inter,ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif}
.school-site *{box-sizing:border-box}.school-site a{color:inherit;text-decoration:none}.school-wrap{width:min(1160px,calc(100% - 48px));margin:0 auto}
.school-header{height:82px;background:rgba(255,255,255,.94);border-bottom:1px solid var(--line);position:sticky;top:0;z-index:30;backdrop-filter:blur(16px)}
.school-header-inner{height:100%;display:flex;align-items:center;justify-content:space-between;gap:24px}
.school-logo-link{display:flex;align-items:center;gap:13px;min-width:0}.school-logo{width:48px;height:48px;object-fit:contain;border-radius:13px;background:#f2f5fb}.school-logo-fallback{width:48px;height:48px;border-radius:13px;background:#eaf0ff;color:var(--blue);display:grid;place-items:center;font-size:23px}
.school-name{font-weight:850;font-size:16px;letter-spacing:-.025em;overflow-wrap:anywhere}.school-kicker{display:block;color:#8792a6;font-size:10px;font-weight:800;letter-spacing:.15em;text-transform:uppercase;margin-bottom:3px}
.school-nav{display:flex;align-items:center;gap:28px;color:#56637a;font-size:13px;font-weight:700}.school-nav a:hover{color:var(--blue)}.school-nav .school-login{padding:12px 19px;background:var(--blue);color:white;border-radius:999px;box-shadow:0 7px 18px #173b8f24}.school-nav .school-login:hover{background:var(--blue-dark);color:white}
.school-menu{display:none;border:1px solid var(--line);background:white;color:var(--ink);border-radius:12px;padding:9px 12px;font-size:13px;font-weight:750}
.school-hero{position:relative;overflow:hidden;background:radial-gradient(ellipse at 82% 20%,#416acb 0,transparent 34%),radial-gradient(ellipse at 8% 100%,#244b9e 0,transparent 42%),linear-gradient(120deg,#0d1d48 0%,#153579 58%,#214a9b 100%);color:white}
.school-hero:after{content:"";position:absolute;inset:0;background-image:linear-gradient(#ffffff08 1px,transparent 1px),linear-gradient(90deg,#ffffff08 1px,transparent 1px);background-size:54px 54px;mask-image:linear-gradient(90deg,black,transparent 85%);pointer-events:none}
.school-hero-inner{position:relative;z-index:1;min-height:490px;padding:90px 0 88px;display:flex;align-items:center}
.school-hero-copy{max-width:760px}.school-eyebrow{display:inline-flex;align-items:center;gap:9px;padding:8px 13px;border:1px solid #ffffff35;border-radius:999px;background:#ffffff12;color:#e1eaff;font-size:10px;font-weight:850;letter-spacing:.16em;text-transform:uppercase}
.school-eyebrow:before{content:"";width:7px;height:7px;background:#8ce0c0;border-radius:50%;box-shadow:0 0 0 4px #8ce0c026}
.school-hero h1{font-size:clamp(42px,6vw,76px);line-height:1.02;letter-spacing:-.065em;margin:25px 0 18px;max-width:850px;overflow-wrap:anywhere}
.school-hero p{font-size:clamp(16px,2vw,19px);line-height:1.8;color:#d4def5;max-width:640px;margin:0}
.school-hero-actions{display:flex;flex-wrap:wrap;gap:12px;margin-top:32px}.school-button{display:inline-flex;align-items:center;justify-content:center;gap:10px;min-height:48px;padding:0 21px;border-radius:999px;font-size:13px;font-weight:800;transition:transform .2s,background .2s}.school-button:hover{transform:translateY(-2px)}.school-button-primary{background:white;color:#153579!important}.school-button-secondary{border:1px solid #ffffff55;color:white!important;background:#ffffff0d}
.school-hero-note{margin-top:45px;display:flex;align-items:center;gap:12px;color:#c4d2ef;font-size:11px}.school-hero-note span{width:34px;height:1px;background:#91a8d8}
.school-main{padding:66px 0 80px}.school-section{margin-top:64px}.school-section:first-child{margin-top:0}
.school-section-head{display:flex;align-items:end;justify-content:space-between;gap:20px;margin-bottom:24px}.school-section-label{color:#5271b4;font-size:10px;letter-spacing:.16em;text-transform:uppercase;font-weight:900;margin:0 0 9px}
.school-section h2{font-size:clamp(25px,3vw,34px);letter-spacing:-.045em;margin:0}.school-section-intro{font-size:13px;line-height:1.7;color:var(--muted);margin:9px 0 0;max-width:560px}
.school-view-all{font-size:12px;color:var(--blue)!important;font-weight:850;white-space:nowrap;padding-bottom:4px}.school-view-all:hover{text-decoration:underline}
.school-feature-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:19px}.school-card{background:white;border:1px solid var(--line);border-radius:19px;overflow:hidden;min-width:0;box-shadow:0 9px 26px #172b4d08;transition:transform .2s,box-shadow .2s}.school-card:hover{transform:translateY(-4px);box-shadow:0 18px 36px #172b4d12}
.school-card-image{width:100%;height:190px;object-fit:cover;display:block;background:#eaf0fa}.school-card-placeholder{height:190px;display:grid;place-items:center;background:linear-gradient(135deg,#eaf0ff,#f5f7fc);color:#6680bd;font-size:32px}
.school-card-body{padding:21px}.school-card-meta{font-size:10px;letter-spacing:.1em;text-transform:uppercase;color:#8190a8;font-weight:850}.school-card h3{font-size:17px;letter-spacing:-.025em;line-height:1.35;margin:10px 0}.school-card p{font-size:12px;line-height:1.7;color:var(--muted);margin:0 0 17px}.school-card-link{font-size:12px;font-weight:850;color:var(--blue)!important}
.school-empty{padding:35px 24px;background:white;border:1px dashed #cbd5e5;border-radius:17px;color:#738098;font-size:13px;text-align:center}
.school-advert{display:grid;grid-template-columns:1fr 1fr;background:white;border:1px solid var(--line);border-radius:22px;overflow:hidden;box-shadow:0 15px 40px #172b4d0a}.school-advert img{width:100%;height:100%;min-height:260px;max-height:360px;object-fit:cover}.school-advert-copy{padding:38px;display:flex;flex-direction:column;justify-content:center}.school-advert-copy h2{font-size:clamp(24px,3vw,34px);line-height:1.12;letter-spacing:-.045em;margin:9px 0 12px}.school-advert-copy p{color:var(--muted);font-size:13px;line-height:1.8;margin:0}.school-advert-copy a{align-self:flex-start;margin-top:21px;color:var(--blue);font-size:12px;font-weight:850}
.school-community-cta{margin-top:76px;padding:38px 42px;border-radius:23px;background:linear-gradient(115deg,#12285f,#1e489d);color:white;display:flex;align-items:center;justify-content:space-between;gap:25px;overflow:hidden;position:relative}.school-community-cta:after{content:"";position:absolute;width:270px;height:270px;border:1px solid #ffffff20;border-radius:50%;right:20%;top:-170px;box-shadow:0 0 0 32px #ffffff08,0 0 0 64px #ffffff05}
.school-community-cta>div,.school-community-cta>a{position:relative;z-index:1}.school-community-cta h2{font-size:clamp(22px,3vw,30px);letter-spacing:-.04em;margin:0 0 8px}.school-community-cta p{color:#cbd8f1;font-size:12px;line-height:1.7;margin:0;max-width:510px}
.school-footer{background:#101d3b;color:#d8e1f5;padding:30px 0}.school-footer-inner{display:flex;align-items:center;justify-content:space-between;gap:20px}.school-footer strong{font-size:13px}.school-footer span{font-size:11px;color:#aab8d5}.school-footer a{color:#fff;font-weight:750}
.school-loading,.school-error{min-height:65vh;display:grid;place-items:center;padding:24px}.school-loading-card,.school-error-card{background:white;border:1px solid var(--line);border-radius:18px;padding:25px;max-width:460px;width:100%;text-align:center;box-shadow:0 12px 35px #172b4d0b}.school-loading-card p,.school-error-card p{color:var(--muted);font-size:13px;line-height:1.6}.school-retry{border:0;border-radius:999px;background:var(--blue);color:white;padding:12px 20px;font-weight:800;cursor:pointer}
@media(max-width:820px){.school-feature-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.school-hero-inner{min-height:430px;padding:76px 0}.school-advert-copy{padding:28px}}
@media(max-width:620px){.school-wrap{width:min(100% - 32px,1160px)}.school-header{height:72px}.school-logo,.school-logo-fallback{width:41px;height:41px}.school-name{font-size:14px}.school-menu{display:block}.school-nav{display:none;position:absolute;top:71px;left:0;right:0;padding:15px 20px 20px;background:white;border-bottom:1px solid var(--line);box-shadow:0 14px 20px #14213d12;align-items:stretch;gap:0;flex-direction:column}.school-nav.is-open{display:flex}.school-nav a{padding:13px 8px}.school-nav .school-login{text-align:center;margin-top:8px}.school-hero-inner{min-height:440px;padding:68px 0}.school-hero h1{font-size:clamp(40px,12vw,58px)}.school-hero p{font-size:15px}.school-main{padding:48px 0 58px}.school-section{margin-top:48px}.school-section-head{align-items:flex-start}.school-feature-grid{grid-template-columns:1fr}.school-card-image,.school-card-placeholder{height:205px}.school-advert{grid-template-columns:1fr}.school-advert img{min-height:190px;height:210px}.school-advert-copy{padding:25px}.school-community-cta{padding:27px 23px;align-items:flex-start;flex-direction:column;margin-top:55px}.school-footer-inner{align-items:flex-start;flex-direction:column}.school-footer span{line-height:1.6}}

.school-contact{margin-top:68px;display:grid;grid-template-columns:.85fr 1.15fr;gap:35px;align-items:start}.school-contact-copy{padding:14px 0}.school-contact-copy h2{font-size:clamp(27px,3vw,38px);line-height:1.12;letter-spacing:-.05em;margin:0 0 13px}.school-contact-copy p{font-size:13px;line-height:1.85;color:var(--muted);margin:0}.school-contact-card{background:white;border:1px solid var(--line);border-radius:21px;padding:27px;box-shadow:0 12px 32px #172b4d0a}.school-contact-grid{display:grid;grid-template-columns:1fr 1fr;gap:15px}.school-contact-card label{display:flex;flex-direction:column;gap:7px;font-size:11px;font-weight:800;color:#43516a}.school-contact-card input,.school-contact-card select,.school-contact-card textarea{width:100%;border:1px solid #dce3ef;border-radius:10px;background:#fbfcfe;padding:12px 13px;font:inherit;font-size:13px;color:var(--ink);outline:none}.school-contact-card input:focus,.school-contact-card select:focus,.school-contact-card textarea:focus{border-color:#5578c3;box-shadow:0 0 0 3px #5578c31c}.school-contact-card textarea{resize:vertical;min-height:125px}.school-contact-wide{grid-column:1/-1}.school-contact-consent{display:flex!important;flex-direction:row!important;align-items:flex-start;gap:10px!important;font-weight:550!important;line-height:1.6}.school-contact-consent input{width:16px;height:16px;margin-top:2px;flex:0 0 auto}.school-contact-submit{border:0;cursor:pointer;background:var(--blue);color:white;border-radius:999px;min-height:47px;padding:0 21px;font-size:12px;font-weight:850}.school-contact-submit:disabled{opacity:.6;cursor:wait}.school-contact-feedback{font-size:12px;line-height:1.6;margin:0 0 15px;padding:11px 13px;border-radius:10px;background:#eef8f3;color:#176044}.school-contact-error{background:#fff1f0;color:#a52a25}.school-honeypot{position:absolute!important;left:-10000px!important;width:1px!important;height:1px!important;overflow:hidden!important}
@media(max-width:720px){.school-contact{grid-template-columns:1fr;gap:20px;margin-top:54px}.school-contact-grid{grid-template-columns:1fr}.school-contact-card{padding:21px}.school-contact-wide{grid-column:auto}}
@media(prefers-reduced-motion:reduce){.school-site *{scroll-behavior:auto!important;transition:none!important}}
`;

export default function SchoolHome() {
  const { slug } = useParams<{ slug: string }>();
  const [data, setData] = useState<HomepageData | null>(null);
  const [error, setError] = useState("");
  const [menuOpen, setMenuOpen] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [enquiryName, setEnquiryName] = useState("");
  const [enquiryEmail, setEnquiryEmail] = useState("");
  const [enquiryPhone, setEnquiryPhone] = useState("");
  const [enquirySubject, setEnquirySubject] = useState("Admissions");
  const [enquiryMessage, setEnquiryMessage] = useState("");
  const [enquiryConsent, setEnquiryConsent] = useState(false);
  const [enquiryWebsite, setEnquiryWebsite] = useState("");
  const [enquiryBusy, setEnquiryBusy] = useState(false);
  const [enquiryFeedback, setEnquiryFeedback] = useState("");
  const [enquiryError, setEnquiryError] = useState("");

  useEffect(() => {
    let active = true;
    setData(null);
    setError("");
    fetch("/backend/public/schools/" + encodeURIComponent(String(slug)) + "/content")
      .then(async (response) => {
        const result = await response.json();
        if (!response.ok) throw new Error(result?.error || "We couldn't find this school website.");
        return result as HomepageData;
      })
      .then((result) => { if (active) setData(result); })
      .catch((reason: unknown) => {
        if (active) setError(reason instanceof Error ? reason.message : "Unable to load this school website.");
      });
    return () => { active = false; };
  }, [slug, attempt]);


  async function submitEnquiry(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setEnquiryBusy(true);
    setEnquiryFeedback("");
    setEnquiryError("");
    try {
      const response = await fetch("/backend/public/schools/" + encodeURIComponent(String(slug)) + "/enquiries", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ full_name: enquiryName, email: enquiryEmail, phone: enquiryPhone, subject: enquirySubject, message: enquiryMessage, consent: enquiryConsent, website: enquiryWebsite }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result?.error?.message || (typeof result?.error === "string" ? result.error : "") || result?.message || "We couldn't submit your enquiry. Please try again.");
      setEnquiryName(""); setEnquiryEmail(""); setEnquiryPhone(""); setEnquirySubject("Admissions"); setEnquiryMessage(""); setEnquiryConsent(false); setEnquiryWebsite("");
      setEnquiryFeedback(result?.message || "Thank you. Your enquiry has been received.");
    } catch (reason) {
      setEnquiryError(reason instanceof Error ? reason.message : "We couldn't submit your enquiry. Please try again.");
    } finally { setEnquiryBusy(false); }
  }

  const school = data?.school;
  const posts = data?.featured_blog ?? [];
  const albums = data?.featured_gallery ?? [];
  const advertisements = data?.advertisements ?? [];

  const formattedPosts = useMemo(() => posts.map((post) => ({
    ...post,
    date: post.published_at
      ? new Intl.DateTimeFormat("en", { month: "short", day: "numeric", year: "numeric" }).format(new Date(post.published_at))
      : "School update",
  })), [posts]);

  if (error) {
    return <main className="school-site"><style>{styles}</style><div className="school-error"><div className="school-error-card"><div aria-hidden="true" style={{fontSize:32}}>⌁</div><h1>School website unavailable</h1><p>{error}</p><button className="school-retry" onClick={() => setAttempt((value) => value + 1)}>Try again</button></div></div></main>;
  }
  if (!data || !school) {
    return <main className="school-site"><style>{styles}</style><div className="school-loading"><div className="school-loading-card"><div aria-hidden="true" style={{fontSize:28}}>✦</div><strong>Preparing the school website</strong><p>Loading the latest school information and updates…</p></div></div></main>;
  }

  const homePath = "/school/" + school.slug;
  return (
    <main className="school-site">
      <style>{styles}</style>
      <header className="school-header">
        <div className="school-wrap school-header-inner">
          <Link href={homePath} className="school-logo-link" aria-label={school.name + " homepage"}>
            {school.logo_url ? <img className="school-logo" src={school.logo_url} alt={school.name + " logo"} /> : <span className="school-logo-fallback" aria-hidden="true">⌂</span>}
            <span><small className="school-kicker">Official school website</small><span className="school-name">{school.name}</span></span>
          </Link>
          <button className="school-menu" type="button" aria-expanded={menuOpen} aria-controls="school-navigation" onClick={() => setMenuOpen((open) => !open)}>{menuOpen ? "Close ✕" : "Menu ☰"}</button>
          <nav id="school-navigation" className={"school-nav" + (menuOpen ? " is-open" : "")} aria-label="School navigation">
            <Link href={homePath + "/blog"} onClick={() => setMenuOpen(false)}>News & updates</Link>
            <Link href={homePath + "/gallery"} onClick={() => setMenuOpen(false)}>Gallery</Link>
            <Link href={homePath + "/login"} className="school-login" onClick={() => setMenuOpen(false)}>School portal <span aria-hidden="true">↗</span></Link>
          </nav>
        </div>
      </header>

      <section className="school-hero">
        <div className="school-wrap school-hero-inner">
          <div className="school-hero-copy">
            <span className="school-eyebrow">Welcome to our school community</span>
            <h1>{school.name}</h1>
            <p>Discover the people, stories and moments that make our school community special. Explore the latest updates, school highlights and shared experiences.</p>
            <div className="school-hero-actions">
              <Link className="school-button school-button-primary" href={homePath + "/blog"}>Explore school updates <span aria-hidden="true">→</span></Link>
              <Link className="school-button school-button-secondary" href={homePath + "/login"}>Access school portal <span aria-hidden="true">↗</span></Link>
            </div>
            <div className="school-hero-note"><span /> A place to learn, connect and grow together</div>
          </div>
        </div>
      </section>

      <div className="school-main school-wrap">
        {advertisements.map((ad) => (
          <section className="school-section" key={ad.id} aria-label="Featured announcement">
            <div className="school-advert">
              <img src={ad.image_url} alt={ad.title} />
              <div className="school-advert-copy">
                <span className="school-section-label">Featured announcement</span>
                <h2>{ad.title}</h2>
                {ad.description && <p>{ad.description}</p>}
                {ad.target_url && <a href={ad.target_url} target="_blank" rel="noopener noreferrer">{ad.button_text || "Find out more"} <span aria-hidden="true">↗</span></a>}
              </div>
            </div>
          </section>
        ))}

        <section className="school-section" id="school-news">
          <div className="school-section-head">
            <div><p className="school-section-label">From our community</p><h2>Latest news & stories</h2><p className="school-section-intro">Keep up with published school news, announcements and moments worth sharing.</p></div>
            <Link className="school-view-all" href={homePath + "/blog"}>View all updates ↗</Link>
          </div>
          {formattedPosts.length ? <div className="school-feature-grid">
            {formattedPosts.map((post) => (
              <article className="school-card" key={post.id}>
                {post.featured_image_url ? <img className="school-card-image" src={post.featured_image_url} alt={post.title} loading="lazy" /> : <div className="school-card-placeholder" aria-hidden="true">✦</div>}
                <div className="school-card-body"><span className="school-card-meta">{post.date}</span><h3>{post.title}</h3><p>{post.excerpt || String(post.content || "").replace(/<[^>]*>/g, " ").slice(0, 150)}</p><Link className="school-card-link" href={homePath + "/blog/" + post.slug}>Read story <span aria-hidden="true">→</span></Link></div>
              </article>
            ))}
          </div> : <div className="school-empty">There are no published news updates yet. Please check back soon.</div>}
        </section>

        <section className="school-section" id="school-gallery">
          <div className="school-section-head">
            <div><p className="school-section-label">Life at our school</p><h2>School moments</h2><p className="school-section-intro">A glimpse of activities, events and memories from the school community.</p></div>
            <Link className="school-view-all" href={homePath + "/gallery"}>Explore gallery ↗</Link>
          </div>
          {albums.length ? <div className="school-feature-grid">
            {albums.map((album) => (
              <Link className="school-card" key={album.id} href={homePath + "/gallery/" + album.slug}>
                {album.cover_image_url ? <img className="school-card-image" src={album.cover_image_url} alt={album.title} loading="lazy" /> : <div className="school-card-placeholder" aria-hidden="true">▧</div>}
                <div className="school-card-body"><span className="school-card-meta">Photo collection</span><h3>{album.title}</h3>{album.description && <p>{album.description}</p>}<span className="school-card-link">View album <span aria-hidden="true">→</span></span></div>
              </Link>
            ))}
          </div> : <div className="school-empty">The school gallery is being updated. New moments will appear here.</div>}
        </section>

        <section className="school-community-cta">
          <div><h2>Stay connected with {school.name}</h2><p>Visit the school portal for your learning and administration activities, or explore the latest public updates from the school.</p></div>
          <Link className="school-button school-button-primary" href={homePath + "/login"}>Open school portal <span aria-hidden="true">↗</span></Link>
        </section>
      </div>


        <section className="school-contact school-wrap" id="contact">
          <div className="school-contact-copy">
            <p className="school-section-label">Talk to our school</p>
            <h2>How can we help you?</h2>
            <p>Send a message to the school about admissions, fees, visits or general questions. The school team can follow up using the contact details you provide. Please do not include sensitive student records or confidential information in this form.</p>
          </div>
          <form className="school-contact-card" onSubmit={submitEnquiry}>
            {enquiryFeedback && <p className="school-contact-feedback" role="status">{enquiryFeedback}</p>}
            {enquiryError && <p className="school-contact-feedback school-contact-error" role="alert">{enquiryError}</p>}
            <div className="school-contact-grid">
              <label>Full name<input required minLength={2} maxLength={120} autoComplete="name" value={enquiryName} onChange={(event) => setEnquiryName(event.target.value)} placeholder="Your full name" /></label>
              <label>Email address<input required type="email" maxLength={254} autoComplete="email" value={enquiryEmail} onChange={(event) => setEnquiryEmail(event.target.value)} placeholder="you@example.com" /></label>
              <label>Phone number <span style={{fontWeight:500,color:"#8792a6"}}>(optional)</span><input type="tel" maxLength={40} autoComplete="tel" value={enquiryPhone} onChange={(event) => setEnquiryPhone(event.target.value)} placeholder="+234…" /></label>
              <label>Enquiry type<select value={enquirySubject} onChange={(event) => setEnquirySubject(event.target.value)}><option>Admissions</option><option>General</option><option>Fees</option><option>Visit</option><option>Other</option></select></label>
              <label className="school-contact-wide">Your message<textarea required minLength={10} maxLength={2000} value={enquiryMessage} onChange={(event) => setEnquiryMessage(event.target.value)} placeholder="How can the school help you?" /></label>
              <div className="school-contact-wide" id="privacy-notice">
                <details style={{border:"1px solid #dce3ef",borderRadius:12,padding:"13px 15px",background:"#fbfcfe"}}>
                  <summary style={{cursor:"pointer",fontSize:12,fontWeight:800,color:"var(--blue)"}}>Read the school enquiry privacy notice</summary>
                  <div style={{fontSize:12,lineHeight:1.75,color:"var(--muted)",paddingTop:12}}>
                    <p><strong>Who handles your information?</strong> The school named on this website is responsible for handling this enquiry. The school management platform is used to securely store and display the message to authorised school administrators.</p>
                    <p><strong>What is collected and why?</strong> Your name, email address, optional phone number, enquiry category, message and consent record are collected so the school can review and respond to your enquiry. Do not include student records, health details, identity documents or other sensitive information in this form.</p>
                    <p><strong>Access and sharing.</strong> The enquiry is visible only to authorised users for this school. It is not a student admission application and does not create a student, parent or guardian account. The school should not use these details for unrelated marketing without a separate valid basis.</p>
                    <p><strong>Retention and your choices.</strong> The school should keep the enquiry only for as long as needed to respond and meet its recordkeeping obligations, then securely delete it under its retention schedule. You may contact the school through its published official contact channels to ask about your information, request correction or request deletion. Some records may need to be retained where a legal or safeguarding obligation applies.</p>
                    <p>Submitting this form confirms that you have read this notice and agree to the use described above. This notice should be reviewed and completed by the school before the form is enabled for public use.</p>
                  </div>
                </details>
              </div>
              <label className="school-contact-consent school-contact-wide"><input required type="checkbox" checked={enquiryConsent} onChange={(event) => setEnquiryConsent(event.target.checked)} /><span>I have read the privacy notice and agree that the school may use these details to respond to my enquiry. I understand this is not an admission application.</span></label>
              <label className="school-honeypot" aria-hidden="true">Leave this field empty<input tabIndex={-1} autoComplete="off" value={enquiryWebsite} onChange={(event) => setEnquiryWebsite(event.target.value)} /></label>
              <div className="school-contact-wide"><button className="school-contact-submit" type="submit" disabled={enquiryBusy}>{enquiryBusy ? "Sending enquiry…" : "Send enquiry →"}</button></div>
            </div>
          </form>
        </section>

      <footer className="school-footer"><div className="school-wrap school-footer-inner"><strong>{school.name}</strong><span>School website powered by <Link href="https://stonezdigital.com.ng" target="_blank">Stonez Digital</Link></span></div></footer>
    </main>
  );
}
