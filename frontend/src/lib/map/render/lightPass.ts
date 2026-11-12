// Draws the light map (lightMap.ts) over the map: a texture of one texel per
// tile, stretched over the tiles with linear filtering and multiplied into
// what is drawn (client: CompositionMode::MULTIPLY).
import { compileProgram } from "./gpu";
import { buildLightMap, type LightFloor, type LightGrid } from "./lightMap";
import LIGHT_VERT from "./shaders/light.vert?raw";
import LIGHT_FRAG from "./shaders/light.frag?raw";

/** Texture unit of the light map (the cell indexes use 2 and 3). */
const LIGHT_UNIT = 4;

/**
 * Everything a light map depends on, flat, to tell whether the last one
 * still holds: most frames only move animations. A chunk counts by its
 * object, as a chunk that changed arrives as a new one.
 */
function inputsOf(
	grid: LightGrid,
	floors: LightFloor[],
	ambient: number,
	spriteSize: number,
	viewFloor: number,
): unknown[] {
	const inputs: unknown[] = [grid.x, grid.y, grid.width, grid.height, ambient, spriteSize, viewFloor];
	for (const floor of floors) {
		inputs.push(floor.z, floor.shift, floor.chunks.length, ...floor.chunks, floor.extra?.length ?? 0);
		for (const light of floor.extra ?? []) inputs.push(light.x, light.y, light.intensity, light.color);
	}
	return inputs;
}

export class LightPass {
	private program: WebGLProgram;
	private vao: WebGLVertexArrayObject;
	private texture: WebGLTexture;
	private uniforms: Record<string, WebGLUniformLocation | null> = {};
	private pixels = new Uint8Array(0);
	/** The grid of the last light map; null when the last frame was not darkened. */
	private grid: LightGrid | null = null;
	private textureWidth = 0;
	private textureHeight = 0;
	/** What the light map in the texture was built from (see inputsOf): the same again needs no new one. */
	private inputs: unknown[] = [];

	constructor(
		private gl: WebGL2RenderingContext,
		corners: WebGLBuffer,
	) {
		this.program = compileProgram(gl, LIGHT_VERT, LIGHT_FRAG);
		for (const name of ["u_screen", "u_rect", "u_light"])
			this.uniforms[name] = gl.getUniformLocation(this.program, name);
		this.vao = gl.createVertexArray()!;
		gl.bindVertexArray(this.vao);
		gl.bindBuffer(gl.ARRAY_BUFFER, corners);
		gl.enableVertexAttribArray(0);
		gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
		gl.bindVertexArray(null);
		this.texture = gl.createTexture()!;
		gl.activeTexture(gl.TEXTURE0 + LIGHT_UNIT);
		gl.bindTexture(gl.TEXTURE_2D, this.texture);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
		gl.activeTexture(gl.TEXTURE0);
	}

	/**
	 * Darkens what is drawn by the light of the grid's tiles. (left, top) is
	 * the screen pixel of the grid's top-left corner, tilePx the device
	 * pixels of a tile, viewFloor the floor in view.
	 */
	draw(
		screenWidth: number,
		screenHeight: number,
		grid: LightGrid,
		left: number,
		top: number,
		tilePx: number,
		floors: LightFloor[],
		ambient: number,
		spriteSize: number,
		viewFloor: number,
	): void {
		const gl = this.gl;
		const size = grid.width * grid.height * 4;
		if (this.pixels.length < size) this.pixels = new Uint8Array(size);
		const pixels = this.pixels.subarray(0, size);
		const inputs = inputsOf(grid, floors, ambient, spriteSize, viewFloor);
		const same = inputs.length === this.inputs.length && inputs.every((input, i) => input === this.inputs[i]);
		this.grid = { ...grid };
		gl.activeTexture(gl.TEXTURE0 + LIGHT_UNIT);
		gl.bindTexture(gl.TEXTURE_2D, this.texture);
		if (!same) {
			buildLightMap(pixels, grid, floors, ambient, spriteSize, viewFloor);
			this.inputs = inputs;
			gl.pixelStorei(gl.UNPACK_ALIGNMENT, 1);
			if (grid.width === this.textureWidth && grid.height === this.textureHeight) {
				gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, 0, grid.width, grid.height, gl.RGBA, gl.UNSIGNED_BYTE, pixels);
			} else {
				gl.texImage2D(
					gl.TEXTURE_2D,
					0,
					gl.RGBA8,
					grid.width,
					grid.height,
					0,
					gl.RGBA,
					gl.UNSIGNED_BYTE,
					pixels,
				);
				this.textureWidth = grid.width;
				this.textureHeight = grid.height;
			}
		}
		gl.useProgram(this.program);
		gl.uniform1i(this.uniforms.u_light, LIGHT_UNIT);
		gl.uniform2f(this.uniforms.u_screen, screenWidth, screenHeight);
		gl.uniform4f(this.uniforms.u_rect, left, top, grid.width * tilePx, grid.height * tilePx);
		gl.blendFunc(gl.DST_COLOR, gl.ZERO);
		gl.bindVertexArray(this.vao);
		gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
		gl.bindVertexArray(null);
		gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
		gl.activeTexture(gl.TEXTURE0);
	}

	clear(): void {
		this.grid = null;
	}

	/** The red of the light of a grid tile in the last light map, 255 off the grid or when it was not darkened (client: LightView::getBrightness). */
	brightness(gridX: number, gridY: number): number {
		const grid = this.grid;
		if (!grid || gridX < grid.x || gridY < grid.y || gridX >= grid.x + grid.width || gridY >= grid.y + grid.height)
			return 255;
		return this.pixels[((gridY - grid.y) * grid.width + (gridX - grid.x)) * 4];
	}

	dispose(): void {
		this.gl.deleteTexture(this.texture);
		this.gl.deleteVertexArray(this.vao);
		this.gl.deleteProgram(this.program);
	}
}
