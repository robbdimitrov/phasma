<script lang="ts">
	import { resolve } from '$app/paths';
	import Avatar from '$lib/components/Avatar.svelte';

	let {
		username,
		avatar,
		primary,
		secondary = null,
		size = 'h-11 w-11',
		align = 'center',
		class: extra = '',
		children
	}: {
		username: string;
		avatar: string | null;
		primary: string;
		secondary?: string | null;
		size?: string;
		align?: 'center' | 'start';
		class?: string;
		children?: import('svelte').Snippet;
	} = $props();
</script>

<a
	href={resolve(`/@${username}`)}
	class="group flex min-w-0 gap-3 {align === 'start' ? 'items-start' : 'items-center'} {extra}"
>
	<Avatar {username} {avatar} {size} />
	<span class="min-w-0 flex-1">
		<span
			class="block truncate text-base font-bold text-base-content transition-colors group-hover:text-primary"
		>
			{primary}
		</span>
		{#if secondary}
			<span class="block truncate text-sm text-base-content/60">{secondary}</span>
		{/if}
		{@render children?.()}
	</span>
</a>
