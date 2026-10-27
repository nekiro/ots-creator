export const FLUIDS = [
  [0, "Empty"],
  [1, "Water"],
  [2, "Blood"],
  [3, "Beer"],
  [4, "Slime"],
  [5, "Lemonade"],
  [6, "Milk"],
  [7, "Mana"],
  [10, "Life"],
  [11, "Oil"],
  [13, "Urine"],
  [14, "Coconut milk"],
  [15, "Wine"],
  [19, "Mud"],
  [21, "Fruit juice"],
  [26, "Lava"],
  [27, "Rum"],
  [28, "Swamp"],
  [35, "Tea"],
  [43, "Mead"],
] as const;

export const FLUID_OPTIONS = FLUIDS.map(([value, label]) => ({ value: value as number, label }));

export const DIRECTION_OPTIONS = [
  { value: 0, label: "North" },
  { value: 1, label: "East" },
  { value: 2, label: "South" },
  { value: 3, label: "West" },
];

export const ATTR_TYPE_OPTIONS = [
  { value: 1, label: "Text" },
  { value: 2, label: "Integer" },
  { value: 3, label: "Float" },
  { value: 4, label: "Boolean" },
  { value: 5, label: "Double" },
];

export function parseCount(text: string): number {
  const count = Math.trunc(Number(text));
  return Number.isFinite(count) && count >= 0 ? Math.min(count, 65535) : 0;
}
