// Moves `node` to be a direct child of <body> so it escapes any ancestor's
// `overflow: hidden` clipping — CSS clips descendants regardless of their own
// `position`, so only leaving the DOM subtree actually avoids that.
export function portal(node: HTMLElement) {
	document.body.appendChild(node);
	return {
		destroy() {
			node.remove();
		}
	};
}
