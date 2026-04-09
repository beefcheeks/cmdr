/**
 * Composites image blocks (images + annotations, sketches) into flat PNGs.
 * Used before dispatching a directive to Claude — converts rich blocks
 * into clean image references that Claude can read.
 */

import { getStroke } from 'perfect-freehand';
import { uploadImage } from './api';
import {
	type Block,
	type ImageBlock,
	type StrokeData,
	parseStrokes,
	serializeBlocks
} from './blocks';

/**
 * Process all blocks: composite images/sketches with annotations into
 * flat PNGs, upload them, and return cleaned markdown.
 *
 * Reports progress via callback: (current, total) => void
 */
export async function compositeAndSerialize(
	blocks: Block[],
	onprogress?: (current: number, total: number) => void
): Promise<string> {
	// Find blocks that need compositing
	const imageBlocks: { index: number; block: ImageBlock }[] = [];
	for (let i = 0; i < blocks.length; i++) {
		const b = blocks[i];
		if (b.type === 'image' && (b.meta || b.path === 'sketch')) {
			imageBlocks.push({ index: i, block: b });
		}
	}

	const total = imageBlocks.length;
	if (total === 0) return serializeBlocks(blocks);

	// Process each image block
	const updated = [...blocks];
	for (let i = 0; i < imageBlocks.length; i++) {
		onprogress?.(i + 1, total);
		const { index, block } = imageBlocks[i];
		const strokes = parseStrokes(block.meta);

		let blob: Blob;
		if (block.path === 'sketch') {
			blob = await renderSketch(strokes);
		} else {
			blob = await renderAnnotatedImage(block.path, strokes);
		}

		// Upload the composited image
		const { url } = await uploadImage(blob);

		// Update the block — clean path, no meta
		updated[index] = {
			...block,
			path: url,
			meta: '',
			caption: block.caption || (block.path === 'sketch' ? 'sketch' : block.caption)
		};
	}

	return serializeBlocks(updated);
}

/**
 * Render a sketch (strokes on white background) to a PNG blob.
 */
async function renderSketch(strokes: StrokeData[]): Promise<Blob> {
	// Determine bounds from strokes
	let maxX = 800, maxY = 450; // minimum size
	for (const stroke of strokes) {
		for (const p of stroke.points) {
			if (p[0] > maxX) maxX = p[0];
			if (p[1] > maxY) maxY = p[1];
		}
	}

	const width = Math.ceil(maxX + 20);
	const height = Math.ceil(maxY + 20);

	const canvas = document.createElement('canvas');
	canvas.width = width;
	canvas.height = height;
	const ctx = canvas.getContext('2d')!;

	// White background
	ctx.fillStyle = '#f0ebe4'; // bourbon-100, matches the sketch bg
	ctx.fillRect(0, 0, width, height);

	// Draw strokes
	drawStrokes(ctx, strokes);

	return canvasToBlob(canvas);
}

/**
 * Render an image with annotation strokes composited on top.
 */
async function renderAnnotatedImage(imagePath: string, strokes: StrokeData[]): Promise<Blob> {
	// Load the original image
	const img = await loadImage(imagePath);

	const canvas = document.createElement('canvas');
	canvas.width = img.naturalWidth;
	canvas.height = img.naturalHeight;
	const ctx = canvas.getContext('2d')!;

	// Draw original image
	ctx.drawImage(img, 0, 0);

	// Scale strokes from display size to natural size
	// The annotations were drawn at display size, but the image may be larger
	const scaleX = img.naturalWidth / img.width;
	const scaleY = img.naturalHeight / img.height;

	if (strokes.length > 0) {
		ctx.save();
		ctx.scale(scaleX, scaleY);
		drawStrokes(ctx, strokes);
		ctx.restore();
	}

	return canvasToBlob(canvas);
}

/**
 * Draw perfect-freehand strokes onto a canvas context.
 */
function drawStrokes(ctx: CanvasRenderingContext2D, strokes: StrokeData[]) {
	for (const stroke of strokes) {
		const outline = getStroke(stroke.points, {
			size: stroke.size,
			thinning: 0.5,
			smoothing: 0.5,
			streamline: 0.5,
		});

		if (outline.length === 0) continue;

		ctx.beginPath();
		ctx.fillStyle = stroke.color;
		ctx.globalAlpha = 0.85;

		const [first, ...rest] = outline;
		ctx.moveTo(first[0], first[1]);
		for (let i = 0; i < rest.length; i++) {
			const [x, y] = rest[i];
			if (i > 0) {
				const [px, py] = rest[i - 1];
				const mx = (px + x) / 2;
				const my = (py + y) / 2;
				ctx.quadraticCurveTo(px, py, mx, my);
			} else {
				ctx.lineTo(x, y);
			}
		}
		ctx.closePath();
		ctx.fill();
	}
	ctx.globalAlpha = 1;
}

/**
 * Load an image from a URL and wait for it to be ready.
 */
function loadImage(src: string): Promise<HTMLImageElement> {
	return new Promise((resolve, reject) => {
		const img = new Image();
		img.crossOrigin = 'anonymous';
		// Handle both API URLs and absolute paths
		if (src.startsWith('/api/')) {
			img.src = src;
		} else {
			img.src = `/api/images/${src.split('/').pop()}`;
		}
		img.onload = () => resolve(img);
		img.onerror = reject;
	});
}

/**
 * Convert a canvas to a PNG blob.
 */
function canvasToBlob(canvas: HTMLCanvasElement): Promise<Blob> {
	return new Promise((resolve, reject) => {
		canvas.toBlob(blob => {
			if (blob) resolve(blob);
			else reject(new Error('Canvas toBlob failed'));
		}, 'image/png');
	});
}
