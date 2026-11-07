export interface Size {
	width: number;
	height: number;
}

// A home screen app with a translucent status bar fills the screen: shorter than it, WebKit kept the keyboard's height.
// Unknown when the window is neither the screen's width nor its height (split view, zoom): never short then.
export function keptShort(inner: Size, screen: Size): boolean {
	const expected = inner.width === screen.width ? screen.height : inner.width === screen.height ? screen.width : 0;
	return expected > 0 && inner.height < expected - 4;
}

// A navigation leaves the page when its path changes or it leaves the app; a search writing its query in the URL does not.
export function leavesPage(from: URL | null | undefined, to: URL | null | undefined): boolean {
	return !from || !to || from.pathname !== to.pathname;
}
