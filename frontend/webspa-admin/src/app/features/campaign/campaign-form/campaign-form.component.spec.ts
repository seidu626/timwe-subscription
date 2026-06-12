import { FormBuilder } from '@angular/forms';
import { convertToParamMap } from '@angular/router';
import { ElementRef } from '@angular/core';
import { of } from 'rxjs';

import { CampaignFormComponent } from './campaign-form.component';

describe('CampaignFormComponent', () => {
  function createComponent(routeSlug?: string) {
    const campaignService = {
      getCampaignBySlug: jasmine.createSpy().and.returnValue(of({
        slug: routeSlug ?? 'campaign-a',
        channel_id: 'channel-1',
        language: 'en',
        country: 'GH',
        operator: 'AirtelTigo',
        offer_product_id: 123,
        flow_type: 'OTP',
        consent_required: true,
        enabled: true
      })),
      updateCampaign: jasmine.createSpy().and.returnValue(of({})),
      createCampaign: jasmine.createSpy().and.returnValue(of({})),
      presignBackgroundUpload: jasmine.createSpy()
    };
    const tenantService = {
      listChannels: jasmine.createSpy().and.returnValue(of({
        channels: [
          {
            channel_id: 'channel-1',
            tenant_id: 'tenant-1',
            channel_key: 'web-gh-airteltigo',
            provider: 'timwe',
            country: 'GH',
            operator: 'AirtelTigo',
            capabilities: ['OTP'],
            status: 'ACTIVE',
            enabled: true,
            created_at: '2026-06-12T00:00:00Z',
            updated_at: '2026-06-12T00:00:00Z'
          }
        ],
        total_count: 1,
        page: 1,
        page_size: 100
      }))
    };

    const router = {
      navigate: jasmine.createSpy()
    };

    const snackBar = {
      open: jasmine.createSpy()
    };

    const elementRef = {
      nativeElement: {
        querySelectorAll: jasmine.createSpy().and.returnValue([]),
        querySelector: jasmine.createSpy().and.returnValue(null)
      }
    } as unknown as ElementRef;

    const route = {
      snapshot: {
        paramMap: convertToParamMap(routeSlug ? { slug: routeSlug } : {})
      }
    };

    const component = new CampaignFormComponent(
      new FormBuilder(),
      campaignService as any,
      tenantService as any,
      route as any,
      router as any,
      snackBar as any,
      elementRef
    );

    component.ngOnInit();

    return { component, campaignService, router, tenantService };
  }

  it('confirms before discarding dirty changes from cancel', () => {
    const { component, router } = createComponent();
    spyOn(window, 'confirm').and.returnValue(false);

    component.form.patchValue({ country: 'NG' });
    component.form.markAsDirty();
    component.onCancel();

    expect(window.confirm).toHaveBeenCalled();
    expect(router.navigate).not.toHaveBeenCalled();
  });

  it('prevents browser unload when the form has unsaved changes', () => {
    const { component } = createComponent();
    const event = {
      preventDefault: jasmine.createSpy(),
      returnValue: undefined as string | undefined
    };

    component.form.patchValue({ country: 'NG' });
    component.form.markAsDirty();
    component.handleBeforeUnload(event as unknown as BeforeUnloadEvent);

    expect(event.preventDefault).toHaveBeenCalled();
    expect(event.returnValue).toBe('');
  });

  it('serializes channel_id when updating a campaign', () => {
    const { component, campaignService } = createComponent('campaign-a');

    component.onSubmit();

    expect(campaignService.updateCampaign).toHaveBeenCalledWith(
      'campaign-a',
      jasmine.objectContaining({ channel_id: 'channel-1' })
    );
  });
});
