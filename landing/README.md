# mcpscope landing — waitlist skeleton

Static HTML for the v0.1.0 pre-release waitlist. **Branch: `landing-v0` only — do not merge to `main` until the customer dev sprint closes (2026-05-12).**

## Enable GitHub Pages in 5 minutes

1. **Go to repo Settings → Pages:** [github.com/sagetta1/mcpscope/settings/pages](https://github.com/sagetta1/mcpscope/settings/pages)
2. **Source:** *Deploy from a branch*
3. **Branch:** `landing-v0` · **Folder:** `/landing`
4. Save. GitHub builds the page in ~30 seconds.
5. **Live URL:** `https://sagetta1.github.io/mcpscope/`

If you prefer a custom domain (e.g. `mcpscope.dev`), add a `landing/CNAME` file with the domain and configure DNS A-records to GitHub Pages IPs.

## Plug in the waitlist form

The page references a Tally embed by ID. Two options:

### Option A — Tally (recommended, no signup needed for free tier)
1. Go to [tally.so/forms/new](https://tally.so/forms/new) and create a one-field form (just an email).
2. Copy the form ID from the URL (e.g. `mYpqrs`).
3. In `landing/index.html` replace `TALLY_FORM_ID` (single occurrence) with that ID.
4. Commit, push.

### Option B — Formspree
1. Sign up at [formspree.io](https://formspree.io), create an endpoint.
2. Replace the entire `<div class="waitlist">…</div>` block in `landing/index.html` with:

```html
<form action="https://formspree.io/f/YOUR_ID" method="POST" class="waitlist">
  <label>Email <input type="email" name="email" required></label>
  <button type="submit">Notify me when v0.1.0 ships</button>
</form>
```

Both routes work without a backend server. Tally is faster to set up (no email verification for sub-50 submissions); Formspree is simpler HTML.

## Replace the demo placeholder

`landing/index.html` shows a `<div class="demo-frame">` placeholder. Before publishing, record a 45-60s screencast of:

```
mcpscope install --apply  →  restart Claude  →  trigger a tool call  →  ./mcpscope ui  →  click session → see traffic
```

Tools: Asciinema (terminal-only) or [LICEcap](https://www.cockos.com/licecap/) / [Kap](https://getkap.co/) (full window). Save as `landing/demo.gif` and replace the placeholder div with `<img src="demo.gif" alt="mcpscope demo">`.

## Decommission

Once the customer dev sprint closes (2026-05-12) and a go/no-go is decided:

- **GO** → merge into main, move `/landing` → standalone repo or keep as Pages source.
- **NO-GO** → close PR, delete branch (`git branch -D landing-v0` + `git push origin --delete landing-v0`), disable Pages.
