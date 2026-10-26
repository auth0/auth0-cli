// Default URL slug for a portal name, for example "Acme Admin" -> "acme-admin".
export function toPortalSlug(name: string): string {
  const slug = name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .replace(/^[^a-z]+/, "")
    .slice(0, 100)
    .replace(/-+$/, "");
  return slug || "my-portal";
}
