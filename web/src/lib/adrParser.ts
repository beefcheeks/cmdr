/**
 * Parses an ADR markdown document into structured sections.
 *
 * Expected format:
 *   # ADR-NNNN: Feature Name
 *   ## Context
 *   ...
 *   ## Approach
 *   ...
 *   ## Architectural Implications
 *   ...
 *   ## Implementation Plan
 *   ...
 */

export interface ADRSection {
	/** Section heading (e.g. "Context", "Approach") */
	heading: string;
	/** Body markdown under the heading */
	body: string;
	/** User annotation if one exists */
	userNote: string | null;
}

export interface ParsedADR {
	/** The H1 title line (e.g. "ADR-0001: Feature Name") */
	title: string;
	/** Everything between the title and the first ## section */
	preamble: string;
	/** Parsed ## sections */
	sections: ADRSection[];
}

/**
 * Parse ADR markdown into title + preamble + sections.
 * Returns null if no ## sections are found.
 */
export function parseADR(md: string): ParsedADR | null {
	const lines = md.split('\n');

	// Find H1 title
	let titleIdx = -1;
	let title = '';
	for (let i = 0; i < lines.length; i++) {
		const m = lines[i].match(/^# (.+)$/);
		if (m) {
			titleIdx = i;
			title = m[1].trim();
			break;
		}
	}

	// Find all ## section starts
	const sectionStarts: { index: number; heading: string }[] = [];
	for (let i = 0; i < lines.length; i++) {
		const m = lines[i].match(/^## (.+)$/);
		if (m) {
			sectionStarts.push({ index: i, heading: m[1].trim() });
		}
	}

	if (sectionStarts.length === 0) return null;

	const preambleStart = titleIdx >= 0 ? titleIdx + 1 : 0;
	const preamble = lines.slice(preambleStart, sectionStarts[0].index).join('\n').trim();

	const sections: ADRSection[] = sectionStarts.map((start, i) => {
		const endIndex = i < sectionStarts.length - 1 ? sectionStarts[i + 1].index : lines.length;
		const body = lines.slice(start.index + 1, endIndex).join('\n').trim();
		const userNote = extractNote(body);

		return {
			heading: start.heading,
			body,
			userNote
		};
	});

	return { title: title || 'Untitled ADR', preamble, sections };
}

const NOTE_RE = /^> Reviewer note:\s*\n((?:> .*(?:\n|$))*)/m;

function extractNote(body: string): string | null {
	const m = body.match(NOTE_RE);
	if (!m) return null;
	return m[1]
		.split('\n')
		.map((l) => l.replace(/^> ?/, ''))
		.join('\n')
		.trim();
}

/**
 * Add or replace a reviewer note on a section.
 */
export function setSectionNote(section: ADRSection, note: string | null): ADRSection {
	let cleanBody = section.body.replace(/\n*> Reviewer note:\s*\n((?:> .*(?:\n|$))*)/, '').trimEnd();

	if (!note) {
		return { ...section, body: cleanBody, userNote: null };
	}

	const noteBlock = '\n\n> Reviewer note:\n' + note.split('\n').map((l) => `> ${l}`).join('\n');
	return {
		...section,
		body: cleanBody + noteBlock,
		userNote: note
	};
}

/**
 * Reconstruct the full ADR markdown from a ParsedADR.
 */
export function reconstructADR(adr: ParsedADR): string {
	const parts = [`# ${adr.title}`];
	if (adr.preamble) parts.push(adr.preamble);
	for (const section of adr.sections) {
		parts.push(`## ${section.heading}\n\n${section.body}`);
	}
	return parts.join('\n\n') + '\n';
}
