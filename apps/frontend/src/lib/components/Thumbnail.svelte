<script lang="ts">
	import { Heart } from '@lucide/svelte';
	import { imageUrl } from '$lib/utils/imageUrl';
	import ImageTile from './ImageTile.svelte';
	import type { Post } from '$lib/types';

	let { post }: { post: Post } = $props();
</script>

<ImageTile postId={post.publicId} src={imageUrl(post.filename)} alt={post.description ?? ''}>
	<div
		class="absolute inset-0 flex items-center justify-center gap-2 bg-black/40 text-sm font-extrabold text-white opacity-0 backdrop-blur-[2px] transition-all duration-300 group-hover:opacity-100"
	>
		<Heart class="h-4 w-4 fill-current text-white" />
		<span>{post.likes}</span>
	</div>
</ImageTile>
