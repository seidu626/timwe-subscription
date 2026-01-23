'use client'

import React, { useMemo, useEffect, useRef } from 'react'
import {
  useExperiment,
  getOrAssignVariant,
  getVariantConfig,
  type Experiment as ExperimentType,
} from '@/app/lib/experiments'

// ============================================
// Types
// ============================================

interface ExperimentProps {
  /** The experiment configuration */
  experiment: ExperimentType | null | undefined
  /** Child components keyed by variant ID */
  children: Record<string, React.ReactNode>
  /** Fallback if no matching variant (defaults to 'control') */
  fallback?: React.ReactNode
  /** Country for targeting */
  country?: string
  /** Callback when variant is assigned */
  onAssignment?: (experimentId: string, variantId: string) => void
}

interface VariantProps {
  /** Variant ID this component renders for */
  id: string
  /** Content to render when this variant is active */
  children: React.ReactNode
}

interface ExperimentContextValue {
  experimentId: string
  variantId: string
  config?: Record<string, unknown>
}

// ============================================
// Context
// ============================================

const ExperimentContext = React.createContext<ExperimentContextValue | null>(null)

/**
 * Hook to access current experiment context
 */
export function useExperimentContext(): ExperimentContextValue | null {
  return React.useContext(ExperimentContext)
}

// ============================================
// Experiment Component
// ============================================

/**
 * Experiment component for A/B testing
 *
 * @example
 * ```tsx
 * <Experiment experiment={ctaExperiment}>
 *   {{
 *     control: <Button color="blue">Subscribe</Button>,
 *     variant_a: <Button color="green">Subscribe Now</Button>,
 *     variant_b: <Button color="orange">Get Started</Button>,
 *   }}
 * </Experiment>
 * ```
 */
export function Experiment({
  experiment,
  children,
  fallback,
  country,
  onAssignment,
}: ExperimentProps): React.ReactNode {
  const hasTracked = useRef(false)

  // Get variant assignment
  const variantId = useMemo(() => {
    if (!experiment) return 'control'
    return getOrAssignVariant(experiment, { country })
  }, [experiment, country])

  // Get variant config
  const config = useMemo(() => {
    if (!experiment) return undefined
    return getVariantConfig(experiment, variantId)
  }, [experiment, variantId])

  // Track assignment
  useEffect(() => {
    if (!experiment || hasTracked.current) return

    if (onAssignment) {
      onAssignment(experiment.id, variantId)
      hasTracked.current = true
    }
  }, [experiment, variantId, onAssignment])

  // Get content for variant
  const content = children[variantId] ?? children['control'] ?? fallback ?? null

  // Provide context for nested components
  const contextValue = useMemo<ExperimentContextValue>(() => ({
    experimentId: experiment?.id ?? '',
    variantId,
    config,
  }), [experiment?.id, variantId, config])

  return (
    <ExperimentContext.Provider value={contextValue}>
      {content}
    </ExperimentContext.Provider>
  )
}

// ============================================
// Variant Component (Alternative API)
// ============================================

/**
 * Variant component for use within Experiment
 * Alternative to object children syntax
 *
 * @example
 * ```tsx
 * <ExperimentWithVariants experiment={myExperiment}>
 *   <Variant id="control">Control content</Variant>
 *   <Variant id="variant_a">Variant A content</Variant>
 * </ExperimentWithVariants>
 * ```
 */
export function Variant({ children }: VariantProps): React.ReactNode {
  // This component is just a marker, actual rendering happens in ExperimentWithVariants
  return children
}

interface ExperimentWithVariantsProps {
  experiment: ExperimentType | null | undefined
  children: React.ReactNode
  country?: string
  onAssignment?: (experimentId: string, variantId: string) => void
}

/**
 * Experiment component that uses Variant children
 */
export function ExperimentWithVariants({
  experiment,
  children,
  country,
  onAssignment,
}: ExperimentWithVariantsProps): React.ReactNode {
  const variantId = useExperiment(experiment, { country, onAssignment })

  // Find matching Variant child
  const variantChildren = React.Children.toArray(children).filter(
    (child): child is React.ReactElement<VariantProps> =>
      React.isValidElement(child) && child.type === Variant
  )

  const matchingVariant = variantChildren.find(
    (child) => child.props.id === variantId
  )

  // Fallback to control variant
  const controlVariant = variantChildren.find(
    (child) => child.props.id === 'control'
  )

  return matchingVariant ?? controlVariant ?? null
}

// ============================================
// Feature Flag Component
// ============================================

interface FeatureFlagProps {
  /** Feature flag name/ID */
  flag: string
  /** Map of flag values to content */
  experiments?: ExperimentType[]
  /** Content when flag is enabled */
  enabled?: React.ReactNode
  /** Content when flag is disabled (default) */
  disabled?: React.ReactNode
  /** Children as fallback */
  children?: React.ReactNode
}

/**
 * Simple feature flag component
 * Checks if a feature is enabled via experiment assignment
 */
export function FeatureFlag({
  flag,
  experiments = [],
  enabled,
  disabled,
  children,
}: FeatureFlagProps): React.ReactNode {
  const flagExperiment = experiments.find(e => e.id === flag)

  const variantId = useMemo(() => {
    if (!flagExperiment) return 'control'
    return getOrAssignVariant(flagExperiment, {})
  }, [flagExperiment])

  const isEnabled = variantId !== 'control' && variantId !== 'disabled'

  if (isEnabled) {
    return enabled ?? children ?? null
  }

  return disabled ?? null
}

// ============================================
// Exports
// ============================================

export default Experiment
