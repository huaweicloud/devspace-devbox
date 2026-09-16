import { ProtocolError } from "../errors.js";

export type WireObject = Record<string, unknown>;

export function objectValue(
  value: unknown,
  message = "DevBox returned an invalid object response",
): WireObject {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new ProtocolError(message);
  }
  return value as WireObject;
}

export function objectItems(value: unknown): WireObject[] {
  if (!Array.isArray(value)) {
    throw new ProtocolError("DevBox returned an invalid list response");
  }
  return value.map((item) => objectValue(item));
}

export function pick(value: WireObject, ...keys: string[]): unknown {
  for (const key of keys) {
    if (key in value) return value[key];
  }
  throw new ProtocolError(`DevBox response field is missing: ${keys[0]}`);
}

export function pickOr(value: WireObject, fallback: unknown, ...keys: string[]): unknown {
  for (const key of keys) {
    if (key in value) return value[key];
  }
  return fallback;
}

export function stringValue(value: unknown): string {
  if (value === undefined || value === null)
    throw new ProtocolError("DevBox response string is missing");
  return String(value);
}

export function optionalString(value: unknown): string | undefined {
  return value === undefined || value === null || value === "" ? undefined : String(value);
}

export function numberValue(value: unknown): number {
  const number = Number(value);
  if (!Number.isFinite(number)) throw new ProtocolError("DevBox response value is not a number");
  return number;
}

export function optionalNumber(value: unknown): number | undefined {
  return value === undefined || value === null ? undefined : numberValue(value);
}

export function dateValue(value: unknown): Date {
  if (value instanceof Date && !Number.isNaN(value.getTime())) return value;
  let input: number | string = typeof value === "number" ? value * 1000 : String(value);
  if (
    typeof input === "string" &&
    /^\d{4}-\d{2}-\d{2}T/.test(input) &&
    !/(?:Z|[+-]\d{2}:?\d{2})$/i.test(input)
  ) {
    input += "Z";
  }
  const date = new Date(input);
  if (Number.isNaN(date.getTime())) throw new ProtocolError("DevBox returned an invalid timestamp");
  return date;
}

export function optionalDate(value: unknown): Date | undefined {
  return value === undefined || value === null || value === "" ? undefined : dateValue(value);
}

export function stringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.map(String) : [];
}

export function stringRecord(value: unknown): Record<string, string> {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, String(item)]));
}

export function identifier(value: string): string {
  if (!value) throw new TypeError("identifier must not be blank");
  return encodeURIComponent(value);
}

export function checkedTimeout(value: number): number {
  if (!Number.isInteger(value) || value < 0 || value > 3600) {
    throw new RangeError("timeout must be an integer between 0 and 3600 seconds");
  }
  return value;
}
