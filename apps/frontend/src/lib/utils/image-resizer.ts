const DEFAULT_MAX_DIMENSION = 1600;
const MIN_DIMENSION = 320;
const MIN_QUALITY = 0.6;
const QUALITY_STEP = 0.08;
const SCALE_STEP = 0.85;
const OUTPUT_TYPE = 'image/jpeg';

export const supportedUploadMimeTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'];
const RESIZED_TARGET_BYTES = 900 * 1024;

function loadImage(file: File): Promise<HTMLImageElement> {
	return new Promise((resolve, reject) => {
		const url = URL.createObjectURL(file);
		const image = new Image();
		image.onload = () => {
			URL.revokeObjectURL(url);
			resolve(image);
		};
		image.onerror = () => {
			URL.revokeObjectURL(url);
			reject(new Error('Could not read image.'));
		};
		image.src = url;
	});
}

function canvasToBlob(canvas: HTMLCanvasElement, quality: number): Promise<Blob> {
	return new Promise((resolve, reject) => {
		canvas.toBlob(
			(blob) => {
				if (blob) {
					resolve(blob);
					return;
				}
				reject(new Error('Could not resize image.'));
			},
			OUTPUT_TYPE,
			quality
		);
	});
}

function targetSize(width: number, height: number, maxDimension: number) {
	const largestSide = Math.max(width, height);
	if (largestSide <= maxDimension) return { width, height };
	const scale = maxDimension / largestSide;
	return { width: Math.round(width * scale), height: Math.round(height * scale) };
}

function drawImage(image: HTMLImageElement, width: number, height: number) {
	const canvas = document.createElement('canvas');
	const context = canvas.getContext('2d');
	if (!context) throw new Error('Could not resize image.');
	canvas.width = width;
	canvas.height = height;
	context.fillStyle = '#ffffff';
	context.fillRect(0, 0, width, height);
	context.drawImage(image, 0, 0, width, height);
	return canvas;
}

function resizedFileName(fileName: string) {
	const baseName = fileName.replace(/\.[^.]+$/, '');
	return `${baseName || 'upload'}.jpg`;
}

export async function resizeImageForUpload(file: File): Promise<File> {
	if (!supportedUploadMimeTypes.includes(file.type)) {
		throw new Error('Choose a JPEG, PNG, GIF, or WEBP image.');
	}
	const image = await loadImage(file);
	let { width, height } = targetSize(
		image.naturalWidth,
		image.naturalHeight,
		DEFAULT_MAX_DIMENSION
	);
	while (true) {
		const canvas = drawImage(image, width, height);
		let quality = 0.88;
		while (quality >= MIN_QUALITY) {
			const blob = await canvasToBlob(canvas, quality);
			if (blob.size <= RESIZED_TARGET_BYTES) {
				return new File([blob], resizedFileName(file.name), {
					type: OUTPUT_TYPE,
					lastModified: Date.now()
				});
			}
			quality -= QUALITY_STEP;
		}
		if (width <= MIN_DIMENSION || height <= MIN_DIMENSION) break;
		width = Math.round(width * SCALE_STEP);
		height = Math.round(height * SCALE_STEP);
	}
	throw new Error('Choose a smaller image.');
}
