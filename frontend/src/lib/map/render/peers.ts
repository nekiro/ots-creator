// The cursors of the other mappers of a shared map, drawn over the map: a
// frame on the tile they point at with their name, or a tag at the edge of
// the view toward them when they are out of sight.
import type { Peer } from "../collab.svelte";
import { floorShift } from "../view/coords";

export interface PeerView {
	/** Canvas size in device pixels. */
	width: number;
	height: number;
	/** Camera center in world pixels. */
	camX: number;
	camY: number;
	/** Device pixels per world pixel. */
	scale: number;
	floor: number;
	tileSize: number;
	pixelRatio: number;
}

const EDGE_MARGIN = 6;

export function drawPeers(context: CanvasRenderingContext2D, peers: Peer[], view: PeerView): void {
	const pixel = Math.max(1, Math.round(view.pixelRatio));
	const tile = view.tileSize * view.scale;
	context.save();
	context.font = `bold ${11 * pixel}px sans-serif`;
	context.textBaseline = "middle";
	for (const peer of peers) {
		const pos = peer.pos;
		if (!pos) continue;
		const shift = floorShift(pos.z, view.floor);
		const left = ((pos.x - shift) * view.tileSize - view.camX) * view.scale + view.width / 2;
		const top = ((pos.y - shift) * view.tileSize - view.camY) * view.scale + view.height / 2;
		const otherFloor = pos.z !== view.floor;
		const label = otherFloor ? `${peer.name} (floor ${pos.z})` : peer.name;
		const visible = left + tile > 0 && top + tile > 0 && left < view.width && top < view.height;
		if (visible) {
			context.globalAlpha = otherFloor ? 0.45 : 1;
			context.lineWidth = 2 * pixel;
			context.strokeStyle = "rgba(0, 0, 0, 0.7)";
			context.strokeRect(left - pixel, top - pixel, tile + 2 * pixel, tile + 2 * pixel);
			context.strokeStyle = peer.color;
			context.setLineDash(otherFloor ? [4 * pixel, 3 * pixel] : []);
			context.strokeRect(left, top, tile, tile);
			context.setLineDash([]);
			tag(context, label + (peer.brush ? ` · ${peer.brush}` : ""), peer.color, left, top - 9 * pixel, pixel);
		} else if (!otherFloor) {
			// Out of sight: a tag at the edge, toward the peer.
			const margin = EDGE_MARGIN * pixel;
			const x = Math.min(Math.max(left + tile / 2, margin), view.width - margin - 80 * pixel);
			const y = Math.min(Math.max(top + tile / 2, margin + 8 * pixel), view.height - margin - 8 * pixel);
			const arrow = arrowToward(left + tile / 2 - view.width / 2, top + tile / 2 - view.height / 2);
			context.globalAlpha = 0.85;
			tag(context, `${arrow} ${peer.name}`, peer.color, x, y, pixel);
		}
	}
	context.restore();
}

function tag(context: CanvasRenderingContext2D, text: string, color: string, x: number, y: number, pixel: number) {
	const padding = 3 * pixel;
	const width = context.measureText(text).width + 2 * padding;
	const height = 14 * pixel;
	context.fillStyle = "rgba(0, 0, 0, 0.75)";
	context.fillRect(x, y - height / 2, width, height);
	context.fillStyle = color;
	context.fillRect(x, y - height / 2, 2 * pixel, height);
	context.fillText(text, x + padding + pixel, y + pixel / 2);
}

function arrowToward(dx: number, dy: number): string {
	const arrows = ["→", "↘", "↓", "↙", "←", "↖", "↑", "↗"];
	const angle = Math.atan2(dy, dx);
	const index = Math.round(angle / (Math.PI / 4));
	return arrows[(index + 8) % 8];
}
