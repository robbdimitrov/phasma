<script lang="ts">
	import { resolve } from '$app/paths';

	let {
		postId,
		src,
		alt,
		children
	}: {
		postId: string;
		src: string;
		alt: string;
		children?: import('svelte').Snippet;
	} = $props();
</script>

<a
	href={resolve(`/posts/${postId}`)}
	class="group relative block aspect-square w-full overflow-hidden rounded-2xl border border-base-300 bg-base-300/30 shadow-sm transition-all duration-500 hover:shadow-md"
>
	<img
		class="h-full w-full object-cover transition-transform duration-700 group-hover:scale-[1.04]"
		{src}
		{alt}
		loading="lazy"
		decoding="async"
		width="300"
		height="300"
	/>
	{@render children?.()}
</a>
