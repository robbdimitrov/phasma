<script lang="ts">
	import { imageUrl } from '$lib/utils/imageUrl';

	let {
		username,
		avatar,
		size = 'h-11 w-11',
		class: extra = ''
	}: {
		username: string;
		avatar: string | null;
		size?: string;
		class?: string;
	} = $props();
</script>

<span
	class="relative block shrink-0 overflow-hidden rounded-full border border-base-300 {size} {extra}"
>
	<img
		class="h-full w-full object-cover"
		src={imageUrl(avatar)}
		alt={username}
		loading="lazy"
		decoding="async"
	/>
</span>
