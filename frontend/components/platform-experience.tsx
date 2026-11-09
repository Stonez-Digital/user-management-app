"use client";

import { useState } from "react";

const questions = [
  {
    question: "How does a school get started?",
    answer: "Submit your school registration and administrator details. Stonez Digital reviews the request before activating the school workspace. Once approved, your administrator can sign in and complete the guided setup.",
  },
  {
    question: "Can different school roles use the platform?",
    answer: "Yes. School administrators, teachers, students, parents and finance teams have role-specific workspaces and access to the information relevant to their responsibilities.",
  },
  {
    question: "Can we manage academics and finance together?",
    answer: "The platform brings academic sessions, terms, classes, enrolment, teaching assignments, assessments, results, invoices and payment records into one connected school workspace.",
  },
  {
    question: "How can we speak to the Stonez Digital team?",
    answer: "Use the support panel on this page to email the team. Include your school name and a short description of what you need help with so the right person can follow up.",
  },
];

const experiences = [
  {
    label: "School leadership",
    title: "See the whole school clearly.",
    detail: "Bring people, academic structures, finance and operational attention points into one school-aware command centre.",
    points: ["School overview and operational health", "Academic setup and enrolment workflows", "Finance summaries and audit visibility"],
  },
  {
    label: "Teachers",
    title: "Keep teaching workflows connected.",
    detail: "Give teachers a focused workspace for assigned classes, subjects, attendance and assessment activities.",
    points: ["Assigned classes and subjects", "Attendance and assessment workflows", "Student results within teaching context"],
  },
  {
    label: "Families",
    title: "Keep families connected to learning.",
    detail: "Provide dedicated student and parent spaces for the academic information made available to each account.",
    points: ["Student-specific academic access", "Parent access for linked learners", "A clearer view of school updates and records"],
  },
];

export function PlatformExperience() {
  const [active, setActive] = useState(0);
  const [openQuestion, setOpenQuestion] = useState<number | null>(0);
  const current = experiences[active];

  return (
    <>
      <section className="experience-section" aria-labelledby="experience-title">
        <div className="marketing-container">
          <div className="experience-heading">
            <span className="marketing-section-label">A WORKSPACE FOR EVERY ROLE</span>
            <h2 id="experience-title">One platform.<br /><em>Different perspectives.</em></h2>
            <p>Explore how each part of your school can work from a connected, role-aware workspace.</p>
          </div>
          <div className="experience-switcher" role="tablist" aria-label="Explore platform experiences">
            {experiences.map((experience, index) => (
              <button
                key={experience.label}
                type="button"
                role="tab"
                id={`experience-tab-${index}`}
                aria-selected={active === index}
                aria-controls="experience-panel"
                className={active === index ? "experience-tab is-active" : "experience-tab"}
                onClick={() => setActive(index)}
              >
                <span>0{index + 1}</span>{experience.label}
              </button>
            ))}
          </div>
          <div className="experience-panel" id="experience-panel" role="tabpanel" aria-labelledby={`experience-tab-${active}`}>
            <div>
              <span className="experience-panel-kicker">CONNECTED SCHOOL OPERATIONS</span>
              <h3>{current.title}</h3>
              <p>{current.detail}</p>
              <a href="/school-signup" className="experience-link">Explore registration <span aria-hidden="true">↗</span></a>
            </div>
            <ul>{current.points.map((point) => <li key={point}><span aria-hidden="true">✓</span>{point}</li>)}</ul>
          </div>
        </div>
      </section>

      <section className="faq-section" aria-labelledby="faq-title">
        <div className="marketing-container faq-layout">
          <div>
            <span className="marketing-section-label">GOOD TO KNOW</span>
            <h2 id="faq-title">Questions,<br /><em>answered.</em></h2>
            <p>Need more detail before registering your school? Start here or contact our team directly.</p>
            <a href="mailto:info@stonezdigital.com.ng?subject=School%20platform%20enquiry" className="faq-contact">Talk to Stonez Digital ↗</a>
          </div>
          <div className="faq-list">
            {questions.map((item, index) => {
              const isOpen = openQuestion === index;
              return (
                <article className={isOpen ? "faq-item is-open" : "faq-item"} key={item.question}>
                  <h3>
                    <button type="button" aria-expanded={isOpen} aria-controls={`faq-answer-${index}`} onClick={() => setOpenQuestion(isOpen ? null : index)}>
                      <span>{item.question}</span><b aria-hidden="true">{isOpen ? "−" : "+"}</b>
                    </button>
                  </h3>
                  {isOpen && <p id={`faq-answer-${index}`}>{item.answer}</p>}
                </article>
              );
            })}
          </div>
        </div>
      </section>
    </>
  );
}

export function SupportWidget() {
  const [open, setOpen] = useState(false);
  return (
    <div className="support-widget">
      {open && (
        <section className="support-card" aria-label="Customer support options">
          <div className="support-card-head">
            <span className="support-online-dot" />
            <div><strong>Stonez Digital support</strong><small>How can our team help?</small></div>
            <button type="button" className="support-close" aria-label="Close support panel" onClick={() => setOpen(false)}>×</button>
          </div>
          <p>Tell us what you need help with. Include your school name if you already have an account.</p>
          <a className="support-email-link" href="mailto:info@stonezdigital.com.ng?subject=School%20platform%20support">Email the support team <span aria-hidden="true">↗</span></a>
          <a className="support-help-link" href="/#faq-title" onClick={() => setOpen(false)}>Read common questions</a>
          <small className="support-response-note">Support requests are handled by the Stonez Digital team.</small>
        </section>
      )}
      <button type="button" className="support-launcher" aria-expanded={open} aria-label={open ? "Close customer support" : "Open customer support"} onClick={() => setOpen(!open)}>
        <span aria-hidden="true">{open ? "×" : "?"}</span><b>{open ? "Close" : "Help & support"}</b>
      </button>
    </div>
  );
}
