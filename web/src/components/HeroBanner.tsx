import type { HomepageBlock } from '../api'

export function HeroBanner({ block }: { block: HomepageBlock }) {
  return (
    <section className="hero-banner">
      <div className="hero-copy">
        <p className="eyebrow">{block.eyebrow}</p>
        <h1>{block.title}</h1>
        <p className="hero-description">{block.body}</p>
        {block.ctaLabel && block.ctaUrl && <a className="button button-primary" href={block.ctaUrl}>{block.ctaLabel}</a>}
      </div>
      <div className="hero-art" aria-hidden="true"><div className="orb orb-one" /><div className="orb orb-two" /></div>
    </section>
  )
}
