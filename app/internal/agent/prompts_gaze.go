package agent

// GazeTaxonomyPlanPrompt assigns a directory path (category) to every entity /
// concept page produced by ONE ingest batch in a single call, so the whole set
// lands on one coherent tree that reuses existing folders — instead of each page
// inventing its own folders in parallel (which diverges worst on the founding
// batch, when the KB still has no folders to anchor on). The result is applied
// in reduce only to pages that don't already have a category, so user edits and
// previously-filed pages are never churned.
const GazeSummaryPrompt = `You are a gaze editor. Given the following document content, create a structured gaze summary page in Markdown format.

<document>
<content>
{{.Content}}
</content>
</document>

<available_gaze_pages>
{{.ExtractedSlugs}}
</available_gaze_pages>

<instructions>
1. The FIRST line of your output MUST be: SUMMARY: {one sentence, 15-40 words, describing what this document is about — for gaze index listing}
2. After the SUMMARY line, write a comprehensive summary of the document in Markdown format.
3. Include the key facts, arguments, and conclusions.
4. Use proper heading hierarchy (## for sections, ### for subsections).
5. **Gaze-link rule**: The available_gaze_pages list above maps slugs to display names and their aliases (format: "[[slug]] = display name (Aliases: a, b)"). Whenever you mention a name or alias that matches a listed entry, you MUST write it as [[slug|display name]] (e.g. [[entity/zhong-guo|中国]]), NOT as bold (**name**) or bare [[slug]]. Use the EXACT slugs provided — do NOT invent new slugs.
6. **Image rule**: If the document contains <images> tags with <image> elements, you SHOULD include the relevant images in your summary using the Markdown syntax: ![caption](url). Place the images where they are contextually relevant to the text. The URL inside ![caption](url) is an opaque token; reproduce it EXACTLY and VERBATIM, do not alter, shorten, or normalize it.
7. At the end, include a "## Key Takeaways" section with bullet points.
8. Write in {{.Language}}.
9. Keep the summary concise but thorough (500-1500 words depending on document length).
10. **Empty content rule**: If the <content> block above is empty, contains only image references with no extracted text, or otherwise carries no substantive information, output exactly: "SUMMARY: No textual content was extractable from this document." followed by a brief note explaining that the document could not be summarised. Do NOT invent a topic, do NOT guess from any other clue.
</instructions>

Output the SUMMARY line first, then the Markdown content. Do not include any other preamble.`

// GazeKnowledgeExtractPrompt extracts both entities and concepts in a single LLM call.
// Returns a JSON object with "entities" and "concepts" arrays.
// This replaces the former separate GazeEntityExtractPrompt and GazeConceptExtractPrompt.
const GazeKnowledgeExtractPrompt = `You are a knowledge extraction system. Analyze the following document and extract all significant entities AND key concepts.

<document>
<content>
{{.Content}}
</content>
</document>

<previous_slugs>
{{.PreviousSlugs}}
</previous_slugs>

<instructions>
Return a JSON object with two arrays: "entities" and "concepts".
**IMPORTANT: Write ALL names, descriptions, and details in {{.Language}}**.

If the <content> block above is empty, contains only image references with no extracted text, or otherwise carries no substantive information, return {"entities": [], "concepts": []}. Do NOT invent entities or concepts from any other source.

### Slug Continuity Rules
If previous slugs are provided above, you MUST follow these rules:
- If an entity or concept from the previous extraction still exists in the current document, **reuse its exact slug** from the previous list. Do NOT generate a new slug for the same thing.
- If an entity or concept no longer appears in the document, **do NOT include it** in the output.
- Only generate new slugs for entities/concepts that are genuinely new (not present in the previous list).
- This ensures slug stability across document updates.

### Entities (people, organizations, products, places, technologies, events, etc.)
Each entity should have:
- "name": The entity name in {{.Language}} (human-readable)
- "slug": URL-friendly slug, format "entity/<lowercase-hyphenated-name>" (use romanized/pinyin form for non-Latin names). **Reuse previous slug if the entity was extracted before.**
- "aliases": An array of strings representing names that refer to THE EXACT SAME entity. Only include: official abbreviations (e.g. "IBM" for "International Business Machines"), full/short name variants (e.g. "腾讯" for "腾讯控股有限公司"), translations (e.g. "Apple" for "苹果公司"), and well-known alternate names (e.g. "Alphabet" for "Google母公司"). Do NOT include parent categories, related products, generic terms, or broader concepts. Provide [] if none.
- "description": **Index listing summary** — one sentence, 15-40 words, in {{.Language}}. Describes WHAT this entity IS and its role in the document. Must be self-contained (understandable without reading the full page). This will be displayed in the gaze index.
- "details": A 2-5 sentence summary in {{.Language}} of key facts from the document. **Image rule**: If the document contains relevant <image> elements in an <images> tag, include them in the details using Markdown syntax: ![caption](url). The URL inside ![caption](url) is an opaque token; reproduce it EXACTLY and VERBATIM, do not alter, shorten, or normalize it.

Only include entities that are substantively discussed (mentioned at least twice or described in detail). Do NOT include generic terms.

### Concepts (topics, themes, methodologies, theories, etc.)
Each concept should have:
- "name": The concept name in {{.Language}} (human-readable)
- "slug": URL-friendly slug, format "concept/<lowercase-hyphenated-name>" (use romanized/pinyin form for non-Latin names). **Reuse previous slug if the concept was extracted before.**
- "aliases": An array of strings representing names that refer to THE EXACT SAME concept. Only include: official abbreviations (e.g. "RAG" for "Retrieval-Augmented Generation"), full/short name variants, and well-known synonyms used interchangeably in the field. Do NOT include sub-topics, related techniques, broader categories, or implementation details. Provide [] if none.
- "description": **Index listing summary** — one sentence, 15-40 words, in {{.Language}}. Defines WHAT this concept IS. Must be self-contained (understandable without reading the full page). This will be displayed in the gaze index.
- "details": A 2-5 sentence explanation in {{.Language}} as discussed in the document. **Image rule**: If the document contains relevant <image> elements in an <images> tag, include them in the details using Markdown syntax: ![caption](url). The URL inside ![caption](url) is an opaque token; reproduce it EXACTLY and VERBATIM, do not alter, shorten, or normalize it.

Only include concepts that are substantively discussed. Skip trivial or overly generic concepts.

### Deduplication Rules
- If something is a specific named thing (person, company, product, place), put it ONLY in "entities".
- If something is an abstract idea, methodology, or theory, put it ONLY in "concepts".
- Never duplicate items across the two arrays.

### JSON Formatting Rules
- **CRITICAL**: Do NOT use literal newline characters inside JSON string values. If you need a newline in a string, you MUST use the escaped sequence \n.
</instructions>

Output ONLY valid JSON. Example:
{
  "entities": [
    {
      "name": "Acme Corp",
      "slug": "entity/acme-corp",
      "aliases": ["Acme", "Acme Corporation"],
      "description": "A technology company specializing in AI solutions.",
      "details": "Acme Corp was founded in 2020 and has grown to 500 employees. They focus on enterprise AI products and recently launched their flagship RAG platform."
    }
  ],
  "concepts": [
    {
      "name": "Retrieval-Augmented Generation",
      "slug": "concept/retrieval-augmented-generation",
      "aliases": ["RAG"],
      "description": "A technique that combines information retrieval with language model generation.",
      "details": "RAG works by first retrieving relevant documents from a knowledge base using vector similarity search, then feeding those documents as context to an LLM for answer generation."
    }
  ]
}`

// GazeCandidateSlugPrompt (Pass 0 of the chunk-cited pipeline) asks the LLM to
// scan a document and output the SKELETON of all entities/concepts it contains:
// name, slug, aliases, a short description, and a short details tiebreaker.
// The heavy lifting — linking each slug to concrete supporting chunks — is
// done in a second pass (see GazeChunkCitationPrompt). Because this prompt no
// longer has to carry full facts per item, it stays cheap even for long docs.
const GazePageModifySystemPrompt = `You are a gaze editor tasked with updating an existing gaze page. You must process NEW information to add and/or deleted documents whose exclusive contributions must be removed.

### SOURCE GROUNDING & MERGE RULES (CRITICAL):
1. **No Inline Chunk IDs:** Chunk handles such as [c003] are internal processing metadata. NEVER output them in the page body or summary, and remove any legacy inline chunk handles from existing content while editing. Source associations are stored separately by the system.
2. **Mandatory Grounding:** Every newly added factual claim, entity, or numerical value MUST be directly supported by the provided new source chunks, but the final prose must remain clean Markdown without inline chunk IDs.
3. **No Hallucination:** Do not invent, synthesize, or infer any information that is not explicitly present in the provided source chunks. If the new chunks clearly and directly supersede or contradict existing content, update the main text to reflect the newer supported information AND add a brief "Contradictions / Updates" section summarizing the change. If the conflict is ambiguous, unresolved, or not directly supported by the provided chunks, do not overwrite the existing content; instead, add only a "Contradictions / Updates" section describing the conflict.
4. The shared source-context block describes what each source document is about and what kind of document it is. Use it only to calibrate scope, attribution, and tone. Never copy source-context wording into the page as factual evidence.
5. Stable system-owned output, grounding, safety, and factuality rules override any business instructions.

### EDITING AND OUTPUT RULES:
1. You are a COMPILER, not a creative writer. Stay close to the verbatim source wording. You may lightly reorder, deduplicate, and join related sentences, but must not rephrase for style, expand short statements, or invent transitions.
2. Do not over-structure. Introduce a section heading only if the source or existing page uses it. Prefer a single top-level heading, short paragraphs, and flat factual lists over an invented hierarchy.
3. Do not add rhetorical filler such as "aims to provide", "designed to", "旨在帮助", "致力于", or "具有重要意义" unless it appears verbatim in an evidentiary source chunk.
4. Keep self-reported claims scoped and attributed. Do not elevate a resume, product page, announcement, or first-person statement into an industry-wide fact.
5. Preserve existing information that remains valid and on-topic. Maintain the existing page's structure and formatting style where possible.
6. Keep a [[slug|name]] link only when its slug is present in the supplied valid-link list. Never invent a slug and never link a page to itself.
7. Images may be included only from supplied new information. Treat each Markdown image URL as an opaque token and reproduce it exactly without altering, shortening, or normalizing it.
8. The first output line must be "SUMMARY: {one sentence, 15-40 words}", followed immediately by clean Markdown page content.

Output the SUMMARY line first, followed by the updated Markdown content, with no other preamble.`

// GazePageModifyUserPrompt contains the per-batch and per-page data. The document-
// level source context deliberately comes first: all pages generated from one
// source then share the longest possible prefix before page metadata diverges.
const GazePageModifyUserPrompt = `{{if .HasAdditions}}<shared_source_contexts>
{{.SharedSourceContexts}}</shared_source_contexts>
{{end}}

<page_metadata>
  <slug>{{.PageSlug}}</slug>
  <title>{{.PageTitle}}</title>
  <type>{{.PageType}}</type>{{if .PageAliases}}
  <aliases>{{.PageAliases}}</aliases>{{end}}
</page_metadata>

This gaze page is specifically about **{{.PageTitle}}** (a {{.PageType}}). Every statement on the page MUST be directly about this exact {{.PageType}} — not about related, adjacent, or similarly-named things.

<existing_page_content>
{{.ExistingContent}}
</existing_page_content>

{{if .HasAdditions}}
<new_information>
{{.NewContent}}
</new_information>

The <new_information> block above is assembled from VERBATIM source chunks already cited as directly supporting this page. The preceding <shared_source_contexts> block is framing only, not evidence.
{{end}}

{{if .HasRetractions}}
<deleted_documents>
{{.DeletedContent}}
</deleted_documents>

<remaining_source_documents>
{{.RemainingSourcesContent}}
</remaining_source_documents>
{{end}}

<valid_gaze_links>
{{.AvailableSlugs}}
</valid_gaze_links>

<instructions>
1. The FIRST line of your output MUST be: SUMMARY: {one sentence, 15-40 words, describing what this page is about after the update — for gaze index listing}
{{if .HasRetractions}}
2. REMOVE facts/claims that were ONLY sourced from the <deleted_documents> and are NOT present in any <remaining_source_documents> or <new_information>.
{{end}}
{{if .HasAdditions}}
3. ADD and MERGE the facts from <new_information> into the page. You are a COMPILER, not a writer:
   - **CRITICAL CONFLICT CHECK**: First verify that the <new_information> is actually about **{{.PageTitle}}** (as declared in <page_metadata>). If a piece of new info clearly belongs to a DIFFERENT but related thing (e.g., this page is about "Hunyuan Model" but the new info is about "Qwen3"; or this page is about "居民身份证" but the new info is about "工作居住证"), you MUST REJECT that part of the new information and DO NOT add it.
   - If it is genuinely about {{.PageTitle}} and contradicts old content, prefer the newer information.
{{end}}
4. Preserve existing information that is still valid and still about {{.PageTitle}}.
5. Keep [[slug|name]] gaze-link references ONLY if the slug appears in the <valid_gaze_links> list above. Remove any [[slug|name]] whose slug is NOT in that list. Do NOT invent new gaze-link slugs. The page's own slug ({{.PageSlug}}) MUST NOT appear as a [[...]] link inside its own content.
6. Maintain the existing page structure and formatting style. Use "# {{.PageTitle}}" as the top-level heading if the page does not already have one. Do NOT introduce new heading levels beyond what the source or existing page justifies.
{{if .HasRetractions}}
7. If after removing deleted content the page becomes nearly empty and there is no new information to add, output just: "SUMMARY: (empty page)\n# {{.PageTitle}}\n\n*This page's primary source document was removed.*"
{{end}}
8. Write in {{.Language}}.
</instructions>

Output the SUMMARY line first, then the updated Markdown content. Do not include any other preamble.`

// GazeIndexIntroPrompt generates the introduction for a NEW index page (first time only).
const GazeIndexIntroPrompt = `You are a gaze editor. Write a brief introduction for a gaze knowledge base index page.

<document_summaries>
{{.DocumentSummaries}}
</document_summaries>

<instructions>
1. Write a title line starting with "# " that reflects the knowledge domain.
2. Follow with 2-3 sentences describing what this gaze covers, based on the document summaries above.
3. Keep it concise — this is just the header section, the directory listing will be added separately below.
4. Write in {{.Language}}.
</instructions>

Output ONLY the title and introduction paragraph. Do NOT generate any directory listings or page links.`

// GazeIndexIntroUpdatePrompt incrementally updates an existing index introduction.
const GazeIndexIntroUpdatePrompt = `You are a gaze editor. Update the introduction section of a gaze index page to reflect recent changes.

<current_introduction>
{{.ExistingIntro}}
</current_introduction>

<changes>
{{.ChangeDescription}}
</changes>

<document_summaries>
{{.DocumentSummaries}}
</document_summaries>

<instructions>
1. Update the introduction to accurately reflect the current state of the gaze.
2. If documents were added, mention the new topics if they significantly change the gaze's scope.
3. If documents were removed, remove references to those topics if they no longer apply.
4. Keep the same tone, style, and title format as the existing introduction.
5. Keep it concise — 1 title line + 2-3 sentences.
6. Write in {{.Language}}.
</instructions>

Output ONLY the updated title and introduction paragraph. Do NOT generate any directory listings or page links.`

// GazeDeduplicationPrompt asks the LLM to identify duplicate entities/concepts
// between newly extracted items and existing gaze pages.
