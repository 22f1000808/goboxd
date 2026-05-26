## [YYYY-MM-DD] [Context: short description of the problem]

**Prompt:**
<paste either the exact prompt or a paraphrased summary that you sent>

**Response summary:**
<what the AI said — paraphrase or direct quote>

**What we used / didn't use:**
<one sentence: what you took and what you discarded, and why>


GOOD ENTRY
## 2026-05-22 · Designing the language registry loader

**Prompt:**
I have a YAML file with a list of language configs. Each language has an id, optional build step, run command, and limits. I want to load this into a Go struct at startup and validate it. What's the cleanest way to do this without introducing an external library?

**Response summary:**
Suggested using encoding/yaml from the standard library (which doesn't exist — only gopkg.in/yaml.v3 does). Also suggested an approach using a map[string]LanguageConfig keyed by id.

**What we used / didn't use:**
Used the map[string]LanguageConfig pattern — clean for lookup by id. Didn't use the yaml suggestion as-is because encoding/yaml is not in the standard library. Used gopkg.in/yaml.v3 after checking the development  guidelines — testify is allowed for tests; we treated this the same way for config parsing.



BADDDD ENTRY
## 2026-05-22 · Go help

**Prompt:**
How do I write a web server in Go?

**Response summary:**
Explained net/http and how to handle routes.

**What we used / didn't use:**
Used most of it.