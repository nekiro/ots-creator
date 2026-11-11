/**
 * Moves the element to <body>. Use it on position:fixed popups: inside a
 * transformed ancestor (a dialog animating in) fixed positions are relative
 * to that ancestor and clipped by its scrolling content.
 */
export function portal(node: HTMLElement) {
	document.body.appendChild(node);
	return {
		destroy() {
			node.remove();
		},
	};
}
