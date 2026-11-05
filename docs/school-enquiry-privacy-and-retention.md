# School website enquiry privacy and retention policy

## Purpose and scope

This template applies to contact enquiries submitted through a school's public website. It covers the name, email address, optional phone number, enquiry category, message, consent timestamp and operational timestamps stored in the school's enquiry inbox.

Each school is the organisation deciding why it handles these enquiries. The platform provides the technical service used to store and display them to authorised school users. A school should review and adopt this notice, identify its own contact channel and confirm its retention schedule before enabling the public form.

## Public notice

The public form must make clear that:
- information is collected to review and respond to the enquiry;
- the form is not an admission application and does not create a student, parent, guardian or user account;
- only authorised users for that school should access the message;
- people should not submit student records or sensitive personal information through the form;
- the school should not repurpose the details for unrelated marketing without a separate valid basis;
- people can use the school's published official contact channels to ask about their information, request correction or request deletion; and
- deletion may be delayed where a legal, safeguarding, dispute or other legitimate recordkeeping obligation requires retention.

The public notice in the school homepage enquiry form is a baseline template. Before public launch, each school must confirm that it accurately describes its own practices and publish a working privacy contact route.

## Retention and deletion schedule

There is no automatic expiry or background purge for enquiries in this release. Each school must adopt and document a retention period before accepting live enquiries. The school should:
1. review open enquiries regularly and close them when no further action is needed;
2. define a period measured from closure or the last meaningful contact, rather than retaining messages indefinitely;
3. consider **12 months after closure or last meaningful contact** as a proposed operational starting point, not a statutory requirement or legal advice; the school must approve or replace this period based on its obligations;
4. retain a record beyond that period only where a documented legal, safeguarding, dispute or other legitimate hold applies; and
5. securely delete enquiries when the approved period expires and no hold applies.

Authorised school administrators can permanently delete an individual enquiry from the school dashboard. Deletion is tenant-scoped: an administrator cannot delete another school's enquiry. The action is irreversible. The school should confirm the request, check for any applicable hold and follow its approved schedule before deleting. This feature does not currently provide automated bulk expiry, anonymisation, export or a deletion audit trail.

## Access and security

- Access is restricted to authenticated school administrators with the relevant content permissions and the active school context.
- Public submissions are validated, rate-limited and protected by a honeypot.
- The enquiry table has PostgreSQL row-level security enabled and broad public/anonymous/authenticated grants revoked; application access uses the server database connection.
- Do not copy enquiry details into personal spreadsheets, messaging groups or unmanaged devices.
- Staff should use the minimum information necessary to respond and should not request sensitive student information through this form.

## School launch checklist

- [ ] Name the person/team responsible for privacy requests.
- [ ] Add and test the school's published privacy contact channel.
- [ ] Review the public notice against the school's actual practices and applicable law.
- [ ] Approve a written retention period and any lawful exceptions.
- [ ] Train authorised staff to review, close and delete enquiries appropriately.
- [ ] Confirm backup and restoration arrangements, including how deletion requests interact with backup retention.
- [ ] Obtain appropriate school approval before enabling the form for real prospective families.

This document is an operational template, not legal advice. Each school should obtain appropriate review for its own circumstances.
