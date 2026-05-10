import { CanDeactivateFn } from '@angular/router';

export interface PendingChangesAware {
  canDiscardChanges: () => boolean;
}

export const pendingChangesGuard: CanDeactivateFn<PendingChangesAware> = (component) => {
  if (!component || typeof component.canDiscardChanges !== 'function') {
    return true;
  }

  return component.canDiscardChanges();
};
