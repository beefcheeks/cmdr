/**
 * Sound effects using Web Audio API for low-latency, overlapping playback.
 * Audio is decoded once into a buffer, then each play() creates a lightweight
 * buffer source — no DOM elements, no decoding delay.
 */

let ctx: AudioContext | null = null;
const buffers = new Map<string, AudioBuffer>();
const loading = new Map<string, Promise<AudioBuffer>>();

function getContext(): AudioContext {
	if (!ctx) ctx = new AudioContext();
	if (ctx.state === 'suspended') ctx.resume();
	return ctx;
}

async function loadBuffer(src: string): Promise<AudioBuffer> {
	const cached = buffers.get(src);
	if (cached) return cached;

	const inflight = loading.get(src);
	if (inflight) return inflight;

	const promise = fetch(src)
		.then((r) => r.arrayBuffer())
		.then((data) => getContext().decodeAudioData(data))
		.then((buffer) => {
			buffers.set(src, buffer);
			loading.delete(src);
			return buffer;
		});

	loading.set(src, promise);
	return promise;
}

export function playSound(src: string, volume = 0.5) {
	const audioCtx = getContext();
	const cached = buffers.get(src);

	if (cached) {
		fire(audioCtx, cached, volume);
	} else {
		loadBuffer(src).then((buf) => fire(audioCtx, buf, volume));
	}
}

function fire(audioCtx: AudioContext, buffer: AudioBuffer, volume: number) {
	const gain = audioCtx.createGain();
	gain.gain.value = volume;
	gain.connect(audioCtx.destination);

	const source = audioCtx.createBufferSource();
	source.buffer = buffer;
	source.connect(gain);
	source.start();
}

// Preload sounds so first play is instant
export function preload(...srcs: string[]) {
	srcs.forEach(loadBuffer);
}

export const SFX = {
	newCommits: '/nba-draft-sound.mp3',
	hover: '/sfx-hover.mp3',
	click: '/sfx-click.mp3'
} as const;
