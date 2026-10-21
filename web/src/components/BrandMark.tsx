import { Link } from "react-router-dom";

type Props = {
  /** icon = mark only; full = wordmark lockup; wordmark = icon + text */
  variant?: "icon" | "full" | "wordmark";
  className?: string;
  /** Destination when clickable. Pass `null` for a non-link mark. Default `/`. */
  to?: string | null;
  size?: "sm" | "md" | "lg";
};

const iconSize = {
  sm: "h-8 w-8",
  md: "h-10 w-10",
  lg: "h-14 w-14",
} as const;

const fullHeight = {
  sm: "h-9",
  md: "h-12",
  lg: "h-16",
} as const;

export function BrandMark({
  variant = "wordmark",
  className = "",
  to = "/",
  size = "md",
}: Props) {
  const content =
    variant === "full" ? (
      <img
        src="/logos/logo-full.svg"
        alt="WyldeTrace"
        className={`${fullHeight[size]} w-auto max-w-full ${className}`}
      />
    ) : variant === "icon" ? (
      <img
        src="/logos/logo.svg"
        alt="WyldeTrace"
        className={`${iconSize[size]} ${className}`}
      />
    ) : (
      <span className={`inline-flex items-center gap-2.5 ${className}`}>
        <img src="/logos/logo.svg" alt="" className={iconSize[size]} />
        <span className="leading-tight">
          <span className="block font-sans text-base font-extrabold tracking-tight text-zinc-100">
            Wylde<span className="font-normal text-brand">Trace</span>
          </span>
          <span className="block text-[10px] font-medium uppercase tracking-[0.28em] text-slate-500">
            System observability
          </span>
        </span>
      </span>
    );

  if (to == null) return content;
  return (
    <Link
      to={to}
      className="inline-flex shrink-0 outline-none focus-visible:ring-2 focus-visible:ring-brand/50"
    >
      {content}
    </Link>
  );
}
