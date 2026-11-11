import type { Metadata } from "next";
import Link from "next/link";
import { PlatformExperience, SupportWidget } from "../components/platform-experience";

export const metadata: Metadata = {
  title: "School Management Software for Connected School Operations",
  description:
    "Bring school administration, academics, finance, teachers, students and parent engagement together with the Stonez Digital School Management Platform.",
  alternates: { canonical: "https://app.stonezdigital.com.ng/" },
  openGraph: {
    type: "website",
    url: "https://app.stonezdigital.com.ng/",
    title: "One Platform. Every School Operation. | Stonez Digital",
    description:
      "A connected school management platform for administration, academics, finance and the people who make every school work.",
    siteName: "Stonez Digital",
  },
  twitter: {
    card: "summary_large_image",
    title: "One Platform. Every School Operation. | Stonez Digital",
    description:
      "Bring your school's people, academics and operations together in one connected platform.",
  },
};

const features = [
  { number: "01", title: "School administration", description: "Keep school records, people, permissions and day-to-day administration organised in one place.", icon: "⌂" },
  { number: "02", title: "Student management", description: "Manage student profiles, enrolment and the information that supports each learner's school journey.", icon: "◎" },
  { number: "03", title: "Academic operations", description: "Bring sessions, terms, classes, sections, subjects, assessments and report cards into one workflow.", icon: "▤" },
  { number: "04", title: "Teacher workflows", description: "Connect teaching responsibilities with assigned classes, subjects and academic activities.", icon: "✳" },
  { number: "05", title: "Parent engagement", description: "Give parents a dedicated way to access relevant information about their linked students.", icon: "♡" },
  { number: "06", title: "Finance management", description: "Organise invoices, payment records and financial reporting with school-aware controls.", icon: "₦" },
  { number: "07", title: "Communication", description: "Support school communication and notification workflows from a connected workspace.", icon: "↗" },
  { number: "08", title: "Platform oversight", description: "Give platform administrators a separate operational view for school onboarding and platform management.", icon: "⌘" },
];

const roles = [
  { title: "School leaders", detail: "Manage school operations, people, academic structures and administration." },
  { title: "Teachers", detail: "Work with assigned classes, subjects and teaching responsibilities." },
  { title: "Students", detail: "Access a dedicated space for relevant academic information." },
  { title: "Parents & guardians", detail: "Stay connected to the school information available for linked students." },
  { title: "Finance teams", detail: "Work with invoices, payments and school finance records." },
];

const steps = [
  { number: "01", title: "Register your school", detail: "Submit your school and administrator details through the existing onboarding flow." },
  { number: "02", title: "Complete platform review", detail: "Stonez Digital reviews the request before activating the school workspace." },
  { number: "03", title: "Receive your school address", detail: "Your dedicated school address is provisioned as part of the onboarding process." },
  { number: "04", title: "Set up and get started", detail: "After approval, sign in to configure your school and invite the right people." },
];

function Arrow() {
  return <span aria-hidden="true" className="marketing-arrow">↗</span>;
}

function DashboardPreview() {
  return (
    <div className="preview-frame" aria-label="Illustrative preview of the school management workspace">
      <div className="preview-topbar">
        <div className="preview-brand"><span className="preview-brand-mark">S</span><span>School workspace</span></div>
        <span className="preview-live"><i /> Connected workspace</span>
      </div>
      <div className="preview-body">
        <aside className="preview-sidebar" aria-hidden="true">
          <span className="preview-side-active">Overview</span><span>People</span><span>Academics</span><span>Finance</span><span>Communication</span>
          <div className="preview-side-bottom"><span>Settings</span></div>
        </aside>
        <div className="preview-content">
          <div className="preview-heading"><div><span className="preview-kicker">SCHOOL OPERATIONS</span><h2>Good morning, team.</h2><p>Your school, connected in one place.</p></div><span className="preview-date">Workspace overview</span></div>
          <div className="preview-metrics">
            <div><span>People</span><strong>Organised</strong><small>Students · Teachers · Parents</small></div>
            <div><span>Academics</span><strong>Connected</strong><small>Sessions · Classes · Results</small></div>
            <div><span>Finance</span><strong>Structured</strong><small>Invoices · Payments</small></div>
          </div>
          <div className="preview-lower">
            <div className="preview-panel"><div className="preview-panel-head"><strong>Core school workflows</strong><span>Workspace</span></div><div className="preview-workflow"><b className="workflow-icon violet">01</b><span><strong>People & enrolment</strong><small>Profiles and school relationships</small></span><i>→</i></div><div className="preview-workflow"><b className="workflow-icon blue">02</b><span><strong>Academic operations</strong><small>Classes, teaching and assessment</small></span><i>→</i></div><div className="preview-workflow"><b className="workflow-icon amber">03</b><span><strong>Finance administration</strong><small>Invoices and payment records</small></span><i>→</i></div></div>
            <div className="preview-panel preview-side-panel"><div className="preview-panel-head"><strong>One connected system</strong></div><div className="preview-orbit"><span>School</span><i className="orbit-dot dot-one" /><i className="orbit-dot dot-two" /><i className="orbit-dot dot-three" /></div><p>Purpose-built spaces for the people and processes behind every school.</p></div>
          </div>
          <p className="preview-disclaimer">Illustrative interface preview · Live data appears inside your authorised school workspace.</p>
        </div>
      </div>
    </div>
  );
}

export default function HomePage() {
  return (
    <main className="marketing-page">
      <header className="marketing-nav">
        <div className="marketing-container marketing-nav-inner">
          <Link href="/" className="marketing-brand" aria-label="Stonez Digital home">
            <img src="/stonez-digital-logo.svg" alt="Stonez Digital" />
            <span><strong>STONEZ DIGITAL</strong><small>SCHOOL MANAGEMENT</small></span>
          </Link>
          <nav className="marketing-links" aria-label="Main navigation">
            <a href="#platform">Platform</a><a href="#features">Features</a><a href="#solutions">Who it serves</a><a href="#pricing">Pricing</a><a href="#security">Security</a>
          </nav>
          <div className="marketing-nav-actions"><Link className="marketing-signin" href="/login">Sign in</Link><Link className="marketing-button marketing-button-small" href="/school-signup">Get started <Arrow /></Link></div>
          <details className="marketing-mobile-menu">
            <summary aria-label="Open navigation menu"><span /><span /><span /></summary>
            <div className="marketing-mobile-panel"><a href="#platform">Platform</a><a href="#features">Features</a><a href="#solutions">Who it serves</a><a href="#pricing">Pricing</a><a href="#security">Security</a><Link href="/login">Sign in</Link><Link href="/school-signup">Register your school</Link></div>
          </details>
        </div>
      </header>

      <section className="marketing-hero" id="platform">
        <div className="marketing-container marketing-hero-grid">
          <div className="marketing-hero-copy">
            <div className="marketing-eyebrow"><span /> A connected operating system for schools</div>
            <h1>One platform.<br /><em>Every school</em><br />operation.</h1>
            <p className="marketing-hero-lede">Bring your school administration, academics, finance and people together in one secure, connected workspace.</p>
            <div className="marketing-hero-actions"><Link href="/school-signup" className="marketing-button">Register your school <Arrow /></Link><a href="#features" className="marketing-text-link">Explore the platform <span>↓</span></a></div>
            <div className="marketing-hero-footnote"><span className="marketing-check">✓</span> Built for the people and processes that keep schools moving.</div>
          </div>
          <div className="marketing-hero-visual"><div className="marketing-visual-glow" /><DashboardPreview /><div className="marketing-visual-caption"><span className="caption-dot" /> A clearer view of your school operations</div></div>
        </div>
        <div className="marketing-container marketing-proofline"><span>ONE CONNECTED WORKSPACE</span><i /><span>ROLE-AWARE EXPERIENCES</span><i /><span>SCHOOL-SCOPED ACCESS</span><i /><span>BUILT BY STONEZ DIGITAL</span></div>
      </section>

      <section className="marketing-intro section-pad">
        <div className="marketing-container marketing-intro-grid"><div><span className="marketing-section-label">THE PLATFORM</span><h2>Less fragmentation.<br /><em>More room to lead.</em></h2></div><div><p>Running a school means coordinating people, learning, records and resources every day. Stonez Digital brings those essential operations into one platform, so each role can focus on the work that matters.</p><a className="marketing-inline-link" href="#features">See what comes together <Arrow /></a></div></div>
      </section>

      <section className="marketing-features section-pad" id="features">
        <div className="marketing-container">
          <div className="marketing-section-heading"><div><span className="marketing-section-label">CAPABILITIES</span><h2>Everything has<br /><em>its place.</em></h2></div><p>A connected set of school workflows, designed to work together while keeping access appropriate to each role.</p></div>
          <div className="marketing-feature-grid">{features.map((feature) => <article className="marketing-feature" key={feature.number}><div className="marketing-feature-top"><span>{feature.number}</span><b aria-hidden="true">{feature.icon}</b></div><h3>{feature.title}</h3><p>{feature.description}</p><span className="marketing-feature-line" /></article>)}</div>
        </div>
      </section>

      <section className="marketing-roles section-pad" id="solutions">
        <div className="marketing-container marketing-roles-grid"><div className="marketing-roles-intro"><span className="marketing-section-label">MADE FOR THE WHOLE SCHOOL</span><h2>Different roles.<br /><em>One shared direction.</em></h2><p>People should see the tools and information that belong to their responsibilities. The platform supports dedicated experiences for school teams, learners and families.</p></div><div className="marketing-role-list">{roles.map((role, index) => <article key={role.title} className="marketing-role"><span>0{index + 1}</span><div><h3>{role.title}</h3><p>{role.detail}</p></div><b aria-hidden="true">↗</b></article>)}</div></div>
      </section>

      <section className="marketing-onboarding section-pad">
        <div className="marketing-container"><div className="marketing-section-heading"><div><span className="marketing-section-label">GETTING STARTED</span><h2>A clear path<br /><em>from day one.</em></h2></div><p>Your school gets its own dedicated workspace through a guided registration and approval process.</p></div><div className="marketing-steps">{steps.map((step) => <article className="marketing-step" key={step.number}><span>{step.number}</span><div><h3>{step.title}</h3><p>{step.detail}</p></div></article>)}</div><div className="marketing-onboarding-note"><span>School workspaces are activated after platform approval.</span><Link href="/school-signup">Start school registration <Arrow /></Link></div></div>
      </section>

      <section className="marketing-pricing section-pad" id="pricing">
        <div className="marketing-container">
          <div className="marketing-section-heading"><div><span className="marketing-section-label">SIMPLE, TRANSPARENT PRICING</span><h2>A plan that grows<br /><em>with your school.</em></h2></div><p>Start with a free one-term trial, then choose the level of capacity and support that suits your school.</p></div>
          <div className="marketing-pricing-grid">
            <article className="marketing-plan"><div className="marketing-plan-head"><span className="marketing-plan-kicker">TRY THE PLATFORM</span><h3>Free Trial</h3><p>Perfect for testing the platform</p></div><div className="marketing-plan-price"><strong>Free</strong><span>1 term</span></div><Link href="/school-signup" className="marketing-plan-button">Get started <Arrow /></Link><ul><li>Up to 50 students</li><li>Up to 5 teachers</li><li>1 GB storage</li><li>Parent portal</li><li>Email notifications</li><li>Basic support</li></ul></article>
            <article className="marketing-plan"><div className="marketing-plan-head"><span className="marketing-plan-kicker">FOR SMALL SCHOOLS</span><h3>Basic Plan</h3><p>Ideal for small schools</p></div><div className="marketing-plan-price"><strong>₦65,000</strong><span>per term</span></div><Link href="/school-signup" className="marketing-plan-button">Get started <Arrow /></Link><ul><li>Up to 200 students</li><li>Up to 15 teachers</li><li>5 GB storage</li><li>Parent portal</li><li>SMS &amp; email notifications</li><li>Standard support</li></ul></article>
            <article className="marketing-plan marketing-plan-popular"><span className="marketing-plan-popular-label">POPULAR</span><div className="marketing-plan-head"><span className="marketing-plan-kicker">FOR GROWING SCHOOLS</span><h3>Professional Plan</h3><p>Best for growing schools</p></div><div className="marketing-plan-price"><strong>₦100,000</strong><span>per term</span></div><Link href="/school-signup" className="marketing-plan-button">Get started <Arrow /></Link><ul><li>Up to 500 students</li><li>Up to 40 teachers</li><li>20 GB storage</li><li>Online payments</li><li>Custom reports</li><li>Priority support</li><li>Custom branding</li><li>Advanced analytics</li><li>Custom domain support</li></ul></article>
            <article className="marketing-plan"><div className="marketing-plan-head"><span className="marketing-plan-kicker">FOR LARGE INSTITUTIONS</span><h3>Premium Plan</h3><p>For large institutions</p></div><div className="marketing-plan-price"><strong>₦150,000</strong><span>per term</span></div><Link href="/school-signup" className="marketing-plan-button">Get started <Arrow /></Link><ul><li>Up to 1,500 students</li><li>Up to 100 teachers</li><li>50 GB storage</li><li>All Professional features</li><li>API access</li><li>Dedicated support</li><li>Advanced analytics</li><li>Custom integrations</li><li>On-site training</li><li>Custom feature requests</li><li>Website &amp; mobile app customisation</li></ul></article>
          </div>
          <div className="marketing-enterprise"><div><strong>Need more?</strong><p>Talk to us about Enterprise plans with unlimited students and teachers.</p></div><a href="mailto:info@stonezdigital.com.ng?subject=Enterprise%20school%20management%20plan">Contact sales <Arrow /></a></div>
          <p className="marketing-pricing-note">All prices are per academic term. Plan limits and paid-feature activation are subject to the selected subscription and platform provisioning.</p>
        </div>
      </section>

      <section className="marketing-security section-pad" id="security">
        <div className="marketing-container marketing-security-grid"><div><span className="marketing-section-label">SECURITY BY DESIGN</span><h2>Trust is built<br /><em>into the structure.</em></h2><p>Schools need confidence that their information and operations stay within the right boundaries. The platform is built around school-aware access and backend-enforced permissions.</p><Link href="/school-signup" className="marketing-security-link">Explore the platform <Arrow /></Link></div><div className="marketing-security-points"><article><span>01</span><div><h3>School-scoped data access</h3><p>School context is enforced across protected application workflows.</p></div></article><article><span>02</span><div><h3>Role-based permissions</h3><p>Platform and school roles receive distinct, authorised experiences.</p></div></article><article><span>03</span><div><h3>Secure account sessions</h3><p>Authentication and session protections support controlled access.</p></div></article><article><span>04</span><div><h3>Operational audit trails</h3><p>Audit logging supports visibility into important platform and school activity.</p></div></article></div></div>
      </section>

      <PlatformExperience />

      <section className="marketing-cta"><div className="marketing-container marketing-cta-inner"><div><span className="marketing-section-label">YOUR SCHOOL, CONNECTED</span><h2>Give your school<br />a smarter way to <em>operate.</em></h2><p>Bring the people, processes and information behind your school into one connected platform.</p></div><div className="marketing-cta-actions"><Link href="/school-signup" className="marketing-button marketing-button-light">Register your school <Arrow /></Link><Link href="/login" className="marketing-cta-signin">Already have platform access? Sign in</Link></div></div></section>

      <footer className="marketing-footer"><div className="marketing-container"><div className="marketing-footer-main"><Link href="/" className="marketing-brand marketing-footer-brand"><img src="/stonez-digital-logo.svg" alt="Stonez Digital" /><span><strong>STONEZ DIGITAL</strong><small>SCHOOL MANAGEMENT</small></span></Link><p>Technology for better-run schools.<br />One connected workspace for school operations.</p><div className="marketing-footer-links"><a href="#platform">Platform</a><a href="#features">Features</a><a href="#pricing">Pricing</a><a href="#security">Security</a><Link href="/school-signup">Register</Link><Link href="/login">Sign in</Link><a href="mailto:info@stonezdigital.com.ng">Contact Stonez Digital</a></div></div><div className="marketing-footer-bottom"><span>© {new Date().getFullYear()} Stonez Digital. All rights reserved.</span><a href="https://stonezdigital.com.ng" target="_blank" rel="noreferrer">Built by Stonez Digital <Arrow /></a></div></div></footer>
      <SupportWidget />
    </main>
  );
}
