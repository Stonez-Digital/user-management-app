import assert from "node:assert/strict";

const base = new URL(process.env.API_BASE_URL);
const results = [];
const sessions = [];

function record(name) {
  results.push(name);
  console.log(`PASS: ${name}`);
}

async function request(path, { token, method = "GET", body } = {}) {
  const response = await fetch(new URL(path, base), {
    method,
    headers: {
      ...(token ? { authorization: `Bearer ${token}` } : {}),
      ...(body !== undefined ? { "content-type": "application/json" } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(15000),
  });
  const text = await response.text();
  let data;
  try { data = text ? JSON.parse(text) : null; } catch { data = text; }
  return { status: response.status, data };
}

async function login(email, password, schoolCode) {
  const result = await request("/auth/login", {
    method: "POST",
    body: { email, password, school_code: schoolCode },
  });
  assert.equal(result.status, 200, "QA login failed; check account status, credentials, school code, and API target");
  assert.ok(result.data?.access_token, "login response did not include access_token");
  assert.ok(result.data?.refresh_token, "login response did not include refresh_token");
  const session = { token: result.data.access_token, refreshToken: result.data.refresh_token };
  sessions.push(session);
  const me = await request("/me", { token: session.token });
  assert.equal(me.status, 200, "could not load authenticated QA profile");
  assert.equal(me.data?.role, "student", "designated QA account must have student role");
  assert.ok(me.data?.school_id, "QA student must belong to a school");
  return { ...session, profile: me.data };
}

const completeResponses = {
  interests: ["build", "investigate", "create"],
  personality_strengths: ["curious", "creative_thinker"],
  work_style: ["independent", "studio"],
  workplace_preferences: ["behind_scenes", "digital"],
  subjects: ["maths", "computing", "arts"],
  education_experience: ["school_project", "digital_project"],
  goals: ["solve", "make"],
};

async function main() {
  assert.equal(base.protocol, "https:", "API_BASE_URL must use HTTPS");
  const a = await login(process.env.QA_A_EMAIL, process.env.QA_A_PASSWORD, process.env.QA_A_SCHOOL_CODE);
  const b = await login(process.env.QA_B_EMAIL, process.env.QA_B_PASSWORD, process.env.QA_B_SCHOOL_CODE);
  const c = await login(process.env.QA_C_EMAIL, process.env.QA_C_PASSWORD, process.env.QA_C_SCHOOL_CODE);
  assert.notEqual(a.profile.id, b.profile.id, "Students A and B must be different students");
  assert.notEqual(a.profile.id, c.profile.id, "Students A and C must be different students");
  assert.notEqual(b.profile.id, c.profile.id, "Students B and C must be different students");
  assert.equal(a.profile.school_id, b.profile.school_id, "Students A and B must belong to the same school to test cross-student isolation");
  assert.notEqual(a.profile.school_id, c.profile.school_id, "Student C must belong to a different school to test cross-school isolation");
  record("Three QA accounts authenticated: A/B same school, C in a different school");

  const createdA = await request("/student/careerpath/attempts", { token: a.token, method: "POST" });
  assert.equal(createdA.status, 200, "Student A create/resume failed");
  const attemptA = createdA.data?.id;
  assert.ok(attemptA, "Student A response did not include attempt id");
  assert.equal(createdA.data?.status, "in_progress", "Student A expected an editable draft");
  record("Student A created or resumed a draft");

  const saved = await request(`/student/careerpath/attempts/${attemptA}/responses`, {
    token: a.token, method: "PUT", body: { responses: completeResponses },
  });
  assert.equal(saved.status, 200, "Student A save responses failed");
  assert.equal(saved.data?.status, "in_progress", "saving must leave the attempt in progress");
  record("Student A saved complete responses");

  const resumedA = await request("/student/careerpath/attempts", { token: a.token, method: "POST" });
  assert.equal(resumedA.status, 200, "Student A resume request failed");
  assert.equal(resumedA.data?.id, attemptA, "create/resume did not return Student A's existing draft");
  const readA = await request(`/student/careerpath/attempts/${attemptA}`, { token: a.token });
  assert.equal(readA.status, 200, "Student A could not reopen their own attempt");
  assert.deepEqual(readA.data?.responses, completeResponses, "saved answers did not persist on reopen");
  record("Student A refreshed/reopened the same draft with saved answers intact");

  const createdB = await request("/student/careerpath/attempts", { token: b.token, method: "POST" });
  assert.equal(createdB.status, 200, "Student B create/resume failed");
  const attemptB = createdB.data?.id;
  assert.ok(attemptB && attemptB !== attemptA, "Student B must have a distinct attempt");
  record("Student B created or resumed a separate draft in the same school");

  const createdC = await request("/student/careerpath/attempts", { token: c.token, method: "POST" });
  assert.equal(createdC.status, 200, "Student C create/resume failed");
  const attemptC = createdC.data?.id;
  assert.ok(attemptC && attemptC !== attemptA && attemptC !== attemptB, "Student C must have a distinct attempt");
  record("Student C created or resumed a separate draft in another school");

  for (const [label, path, method, body] of [
    ["read", `/student/careerpath/attempts/${attemptB}`, "GET", undefined],
    ["save", `/student/careerpath/attempts/${attemptB}/responses`, "PUT", { responses: completeResponses }],
    ["submit", `/student/careerpath/attempts/${attemptB}/submit`, "POST", undefined],
  ]) {
    const result = await request(path, { token: a.token, method, body });
    assert.ok([403, 404].includes(result.status), `same-school cross-student access: Student A must be denied Student B's ${label}; got HTTP ${result.status}`);
  }
  record("Same-school cross-student isolation: A denied read/save/submit access to B's attempt");

  for (const [label, path, method, body] of [
    ["read", `/student/careerpath/attempts/${attemptA}`, "GET", undefined],
    ["save", `/student/careerpath/attempts/${attemptA}/responses`, "PUT", { responses: completeResponses }],
    ["submit", `/student/careerpath/attempts/${attemptA}/submit`, "POST", undefined],
  ]) {
    const result = await request(path, { token: c.token, method, body });
    assert.ok([403, 404].includes(result.status), `cross-school access: Student C must be denied Student A's ${label}; got HTTP ${result.status}`);
  }
  record("Cross-school isolation: C denied read/save/submit access to A's attempt");

  for (const [label, path, method, body] of [
    ["read", `/student/careerpath/attempts/${attemptC}`, "GET", undefined],
    ["save", `/student/careerpath/attempts/${attemptC}/responses`, "PUT", { responses: completeResponses }],
    ["submit", `/student/careerpath/attempts/${attemptC}/submit`, "POST", undefined],
  ]) {
    const result = await request(path, { token: a.token, method, body });
    assert.ok([403, 404].includes(result.status), `cross-school access: Student A must be denied Student C's ${label}; got HTTP ${result.status}`);
  }
  record("Cross-school isolation: A denied read/save/submit access to C's attempt");

  if (process.env.CONFIRM_IMMUTABLE_SUBMIT === "yes") {
    const submitted = await request(`/student/careerpath/attempts/${attemptA}/submit`, { token: a.token, method: "POST" });
    assert.equal(submitted.status, 200, "Student A submission failed");
    assert.equal(submitted.data?.status, "submitted", "successful submit must return submitted status");
    assert.ok(Array.isArray(submitted.data?.results) && submitted.data.results.length > 0, "submission should persist career results");
    const reopened = await request("/student/careerpath/attempts", { token: a.token, method: "POST" });
    assert.equal(reopened.status, 200, "could not reopen submitted assessment");
    assert.equal(reopened.data?.id, attemptA, "resume should return the submitted attempt");
    assert.equal(reopened.data?.status, "submitted", "reopened assessment must remain submitted");
    assert.deepEqual(reopened.data?.results, submitted.data.results, "results changed after reopening");
    const mutateSubmitted = await request(`/student/careerpath/attempts/${attemptA}/responses`, {
      token: a.token, method: "PUT", body: { responses: completeResponses },
    });
    assert.equal(mutateSubmitted.status, 409, "submitted attempt must be immutable (expected HTTP 409)");
    record("Student A submitted, reopened stable results, and confirmed submitted attempt immutability");
  } else {
    console.log("SKIP: immutable submission not enabled for this run");
  }

  console.log(`CareerPath live smoke checks completed: ${results.length} passed.`);
}

try {
  await main();
} finally {
  for (const session of sessions) {
    try {
      await request("/auth/logout", {
        method: "POST",
        body: { refresh_token: session.refreshToken },
      });
    } catch {
      console.log("NOTICE: logout request did not complete; revoke QA sessions in the account dashboard if needed.");
    }
  }
}
