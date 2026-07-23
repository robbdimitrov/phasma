const maxImageUploadBytes = 1024 * 1024;
const allowedImageTypes = new Set(['image/jpeg', 'image/png', 'image/gif', 'image/webp']);

type ImageUploadValidation =
	{ ok: true; file: File } | { ok: false; status: 400 | 413; error: string };

export function validateImageUpload(file: unknown, requiredMessage: string): ImageUploadValidation {
	if (!(file instanceof File) || file.size === 0) {
		return { ok: false, status: 400, error: requiredMessage };
	}
	if (file.size > maxImageUploadBytes) {
		return { ok: false, status: 413, error: 'Image must be smaller than 1 MB.' };
	}
	if (!allowedImageTypes.has(file.type)) {
		return { ok: false, status: 400, error: 'Please select a JPEG, PNG, GIF, or WEBP image.' };
	}
	return { ok: true, file };
}
