# Documentation style

The provider's users range from identity experts to people who are new to both Terraform and
WSO2. Write every page so that a ninth-grade reader can follow it. This guide applies to the
guides in `templates/`, the schema descriptions in the Go code (they become the resource pages in
`docs/`), the comments in `examples/`, and the Markdown files in the repository.

## Rules

1. **Write complete sentences.** Every sentence has a subject and a verb. Avoid fragments such as
   "Bootstrapping, labs." or "Defaults to X; see Y."
2. **Name the subject.** Say "the provider", "the server", "Terraform" or "the resource" instead of
   "it" or "this" when the reader could guess wrong. Do not chain ideas with colons and semicolons.
   Split them into separate sentences.
3. **Keep sentences short.** Aim for 20 words or fewer. Put one idea in each sentence.
4. **Use everyday words.** Prefer "use" to "utilize", "change" to "mutate", "turn on" to "enable"
   when both work. Explain a technical term the first time a page uses it, for example
   "an M2M (machine-to-machine) application".
5. **Talk to the reader.** Use "you" and the active voice: "Set `tenant_domain` to manage another
   tenant", not "`tenant_domain` may be set in order to ...".
6. **Show steps as numbered lists.** A tutorial says what the reader will build, what they need,
   the steps, how to check the result, and how to clean up.
7. **Say what happens, then what to do.** For a limitation, explain what the server does, what the
   provider does about it, and what the reader should do.

## Schema descriptions

* A resource description starts with one sentence that says what the resource manages. Further
  sentences cover what happens on destroy and how to import, when that is not obvious.
* An attribute description says what the value means, in a sentence or a short noun phrase such
  as "The ID of the group." Add the default and any limits as separate sentences:
  "The default is `true`." "Changing this value replaces the resource."
* Use backticks for attribute names, values and API paths.

## Examples

Before:

> Pick one method. Credentials set in the provider block take precedence, **as a group**, over the
> `WSO2IS_*` environment variables: a block with `client_id`/`client_secret` ignores
> `WSO2IS_USERNAME` and `WSO2IS_PASSWORD`.

After:

> Choose one of these three methods. The provider reads credentials from the provider block first.
> If the block sets any credential, the provider ignores all credential environment variables.
> For example, a provider block that sets `client_id` and `client_secret` ignores
> `WSO2IS_USERNAME` and `WSO2IS_PASSWORD`.
