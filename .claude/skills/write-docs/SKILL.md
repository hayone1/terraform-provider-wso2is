---
name: write-docs
description: Write or edit user documentation for terraform-provider-wso2is (guides, schema descriptions, examples, README) in clear, tutorial-like, ninth-grade English. Use when adding or changing a guide, a resource or attribute description, an example comment, or any Markdown file meant for users.
---

# Write documentation

Follow `contributing/docs-style.md`. The short version:

1. **Complete sentences** with a subject and a verb. No fragments ("Defaults to X."), no chains
   joined by colons and semicolons.
2. **Name the subject** ("the provider", "the server", "Terraform") instead of an unclear "it".
3. **Short sentences**, about 20 words or fewer, one idea each. Explain jargon on first use.
4. **Talk to the reader** in the active voice: "Set `x` to ...".
5. **Tutorials** say what the reader builds, what they need, numbered steps, how to check the
   result, and how to clean up.
6. **Limitations** say what the server does, what the provider does about it, and what the
   reader should do.

## Where documentation comes from

| Output | Source | Regenerate |
|--------|--------|------------|
| `docs/index.md` | `templates/index.md.tmpl` + provider schema | `task docs` |
| `docs/guides/*.md` | `templates/guides/*.md.tmpl` (+ `examples/guides/`) | `task docs` |
| `docs/resources/*.md`, `docs/data-sources/*.md` | `MarkdownDescription` in `internal/service/**` + `examples/resources/<type>/` | `task docs` |
| README, CONTRIBUTING, deploy/README, RELEASING | the Markdown files themselves | none |

Never edit `docs/` by hand; CI fails when `docs/` differs from what `task docs` generates.

## Checklist

- [ ] Every new or changed sentence follows the rules above.
- [ ] Examples are realistic and self-contained; comments explain *why*, in full sentences.
- [ ] `task docs` and `task examples:validate` pass (`task examples:validate TF_CLI=tofu` too).
- [ ] User-visible server quirks are in `templates/guides/server-behaviour.md.tmpl`.
