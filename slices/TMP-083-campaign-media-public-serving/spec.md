# TMP-083: Campaign media public serving

## Problem

Campaign/ad artwork was wired end to end (presign endpoint, console upload,
campaigns.app_artwork_url, artwork_url in the app catalog) but zero prod
campaigns have artwork, because both halves of the storage URL story were
unreachable from outside the docker network:

- Stored asset URLs used CAMPAIGN_ASSET_STORAGE_PUBLIC_BASE_URL, set to
  http://localhost:9100/campaign-assets on the droplet.
- Presigned PUT URLs pointed at the internal storage endpoint
  (http://minio:9000/...), so browser uploads from the admin console could
  never succeed.

## Approach

1. webspa-admin nginx: `location ^~ /assets/` proxies to minio:9000,
   stripping the /assets/ prefix and pinning the Host header to the storage
   endpoint so presign signatures stay valid. ^~ keeps the static-asset
   cache regex from swallowing proxied images. Resolver + variable defer
   DNS so nginx boots even if minio is down.
2. acquisition-api: optional CAMPAIGN_ASSET_STORAGE_PUBLIC_UPLOAD_BASE_URL
   rewrites presigned PUT URLs onto the public vhost (same
   ends-with-bucket convention as PUBLIC_BASE_URL). Unset keeps current
   behavior.
3. Droplet env: both URLs set to
   https://admin.nouveauricheglobalgroup.com/assets/campaign-assets.
   MAX_UPLOAD_BYTES lowered to 1000000 because the host nginx (not
   editable without sudo) caps request bodies at the 1m default; a clean
   validation error beats an opaque 413.

Only future uploads get the fixed URL: asset URLs are stored at presign
time and no existing rows carry artwork, so there is nothing to backfill.

## Verification

- go build; go test CampaignAsset/Presign set (2 new tests: rewrite to
  public base, unset keeps storage endpoint).
- Live: anonymous GET of a bucket object through the public proxy returns
  200; a presigned PUT URL rewritten onto the public host uploads with a
  valid signature and reads back.
