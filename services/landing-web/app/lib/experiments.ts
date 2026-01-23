'use client'

import { useEffect, useMemo, useRef } from 'react'

// ============================================
// Types
// ============================================

export interface ExperimentVariant {
  id: string
  name: string
  weight: number // 0-100
  config?: Record<string, unknown>
}

export interface Experiment {
  id: string
  name: string
  enabled: boolean
  variants: ExperimentVariant[]
  targeting?: {
    countries?: string[]
    devices?: ('mobile' | 'desktop' | 'tablet')[]
    traffic_percent?: number
  }
}

export interface ExperimentAssignment {
  experimentId: string
  variantId: string
  assignedAt: string
}

// ============================================
// Constants
// ============================================

const EXPERIMENT_STORAGE_KEY = 'lp_experiments'
const USER_ID_KEY = 'lp_user_id'

// ============================================
// User ID Management
// ============================================

/**
 * Get or create a persistent user ID for consistent experiment assignment
 */
export function getUserId(): string {
  if (typeof window === 'undefined') return ''

  let userId = localStorage.getItem(USER_ID_KEY)
  if (!userId) {
    userId = `${Date.now()}-${Math.random().toString(36).substring(2, 15)}`
    localStorage.setItem(USER_ID_KEY, userId)
  }
  return userId
}

// ============================================
// Hashing
// ============================================

/**
 * Simple hash function for consistent bucket assignment
 * Uses djb2 algorithm for deterministic results
 */
function hashCode(str: string): number {
  let hash = 5381
  for (let i = 0; i < str.length; i++) {
    hash = (hash << 5) + hash + str.charCodeAt(i)
  }
  return hash >>> 0 // Convert to unsigned 32-bit integer
}

// ============================================
// Device Detection
// ============================================

type DeviceType = 'mobile' | 'desktop' | 'tablet'

function detectDevice(): DeviceType {
  if (typeof window === 'undefined') return 'desktop'

  const ua = navigator.userAgent.toLowerCase()

  // Check for tablets first (before mobile check)
  if (/ipad|tablet|playbook|silk/.test(ua)) {
    return 'tablet'
  }

  // Check for mobile
  if (/mobile|iphone|ipod|android|blackberry|opera mini|iemobile/.test(ua)) {
    return 'mobile'
  }

  return 'desktop'
}

// ============================================
// Targeting
// ============================================

/**
 * Check if user matches experiment targeting criteria
 */
function matchesTargeting(
  experiment: Experiment,
  options: { country?: string } = {}
): boolean {
  const { targeting } = experiment
  if (!targeting) return true

  // Check country targeting
  if (targeting.countries && targeting.countries.length > 0) {
    if (!options.country || !targeting.countries.includes(options.country)) {
      return false
    }
  }

  // Check device targeting
  if (targeting.devices && targeting.devices.length > 0) {
    const device = detectDevice()
    if (!targeting.devices.includes(device)) {
      return false
    }
  }

  // Check traffic percentage
  if (targeting.traffic_percent !== undefined && targeting.traffic_percent < 100) {
    const userId = getUserId()
    const bucket = hashCode(`${experiment.id}-traffic-${userId}`) % 100
    if (bucket >= targeting.traffic_percent) {
      return false
    }
  }

  return true
}

// ============================================
// Variant Assignment
// ============================================

/**
 * Assign a variant to a user for an experiment
 * Uses consistent hashing so the same user always gets the same variant
 */
export function assignVariant(experiment: Experiment, userId: string): string {
  if (!experiment.enabled || experiment.variants.length === 0) {
    return 'control'
  }

  // Calculate hash bucket (0-99)
  const hash = hashCode(`${experiment.id}-${userId}`)
  const bucket = hash % 100

  // Find variant based on cumulative weight
  let cumulative = 0
  for (const variant of experiment.variants) {
    cumulative += variant.weight
    if (bucket < cumulative) {
      return variant.id
    }
  }

  // Fallback to first variant (control)
  return experiment.variants[0]?.id || 'control'
}

// ============================================
// Assignment Storage
// ============================================

/**
 * Get stored experiment assignments
 */
export function getStoredAssignments(): Record<string, ExperimentAssignment> {
  if (typeof window === 'undefined') return {}

  try {
    const stored = localStorage.getItem(EXPERIMENT_STORAGE_KEY)
    if (stored) {
      return JSON.parse(stored)
    }
  } catch {}

  return {}
}

/**
 * Store an experiment assignment
 */
function storeAssignment(assignment: ExperimentAssignment): void {
  if (typeof window === 'undefined') return

  try {
    const assignments = getStoredAssignments()
    assignments[assignment.experimentId] = assignment
    localStorage.setItem(EXPERIMENT_STORAGE_KEY, JSON.stringify(assignments))
  } catch {}
}

/**
 * Get or assign variant for an experiment
 */
export function getOrAssignVariant(
  experiment: Experiment,
  options: { country?: string } = {}
): string {
  // Check if experiment is enabled
  if (!experiment.enabled) {
    return 'control'
  }

  // Check targeting
  if (!matchesTargeting(experiment, options)) {
    return 'control'
  }

  // Check for existing assignment
  const assignments = getStoredAssignments()
  const existing = assignments[experiment.id]
  if (existing) {
    return existing.variantId
  }

  // Assign new variant
  const userId = getUserId()
  const variantId = assignVariant(experiment, userId)

  // Store assignment
  storeAssignment({
    experimentId: experiment.id,
    variantId,
    assignedAt: new Date().toISOString(),
  })

  return variantId
}

// ============================================
// React Hook
// ============================================

interface UseExperimentOptions {
  country?: string
  onAssignment?: (experimentId: string, variantId: string) => void
}

/**
 * Hook to get variant assignment for an experiment
 */
export function useExperiment(
  experiment: Experiment | null | undefined,
  options: UseExperimentOptions = {}
): string {
  const { country, onAssignment } = options
  const hasTracked = useRef(false)

  const variantId = useMemo(() => {
    if (!experiment) return 'control'
    return getOrAssignVariant(experiment, { country })
  }, [experiment, country])

  // Track assignment
  useEffect(() => {
    if (!experiment || hasTracked.current) return

    if (onAssignment) {
      onAssignment(experiment.id, variantId)
      hasTracked.current = true
    }
  }, [experiment, variantId, onAssignment])

  return variantId
}

/**
 * Hook for multiple experiments
 */
export function useExperiments(
  experiments: Experiment[],
  options: UseExperimentOptions = {}
): Record<string, string> {
  const { country } = options

  return useMemo(() => {
    const assignments: Record<string, string> = {}
    for (const experiment of experiments) {
      assignments[experiment.id] = getOrAssignVariant(experiment, { country })
    }
    return assignments
  }, [experiments, country])
}

// ============================================
// Variant Config Helper
// ============================================

/**
 * Get configuration for the assigned variant
 */
export function getVariantConfig<T = Record<string, unknown>>(
  experiment: Experiment,
  variantId: string
): T | undefined {
  const variant = experiment.variants.find(v => v.id === variantId)
  return variant?.config as T | undefined
}

// ============================================
// Debug Utilities
// ============================================

/**
 * Force a specific variant for testing (development only)
 */
export function forceVariant(experimentId: string, variantId: string): void {
  if (typeof window === 'undefined') return
  if (process.env.NODE_ENV !== 'development') return

  storeAssignment({
    experimentId,
    variantId,
    assignedAt: new Date().toISOString(),
  })

  // Reload to apply
  window.location.reload()
}

/**
 * Clear all experiment assignments (development only)
 */
export function clearExperiments(): void {
  if (typeof window === 'undefined') return
  if (process.env.NODE_ENV !== 'development') return

  localStorage.removeItem(EXPERIMENT_STORAGE_KEY)
  window.location.reload()
}

/**
 * Get all current experiment assignments for debugging
 */
export function debugExperiments(): Record<string, ExperimentAssignment> {
  return getStoredAssignments()
}

export default useExperiment
