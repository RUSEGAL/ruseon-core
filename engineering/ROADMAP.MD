# RUSEON Roadmap

## Product direction

RUSEON is an edge-first video management platform for distributed CCTV
installations. An edge node must continue ingesting, recording, serving local
viewers, and producing events when the control plane or WAN is unavailable.
The control plane adds centralised operations; it is not a prerequisite for
local video retention.

The primary product path is an AI VMS for sites with 20--500 cameras, starting
with warehouses and industrial facilities. RUSEON should not attempt to become
a general-purpose media server or a turnkey model-training platform. Its core
value is a dependable video pipeline plus the operator workflow that turns
video and AI metadata into incidents, clips, and evidence.

## Edition boundaries

The Community Edition (CE) is a useful, self-hosted, single-node VMS. It must
remain capable of operating a complete small or medium site without a paid
service. Pro and Enterprise add commercial operations, scale, and governance;
they must not make the CE video path deliberately unusable.

| Area | Community Edition (CE) | Pro | Enterprise |
| --- | --- | --- | --- |
| Deployment | Single self-hosted edge node | Single or multi-site managed edge nodes | Multi-site, clustered deployments |
| Video | RTSP/ONVIF ingest; HLS and WHEP playback; local archive | CE plus compatibility and WAN delivery options | CE and Pro plus high-availability delivery |
| Cameras | Discovery, profiles, health, PTZ where supported | Bulk onboarding and fleet templates | Fleet policy, delegated administration, tenant isolation |
| AI | Open metadata API; reference object-detection connector; local event rules | Certified connectors, model/worker lifecycle, advanced event packs | Central governance, cross-site search, audit and retention policies |
| Storage | Local fMP4 archive, export, retention | Storage reporting and managed backup options | S3/MinIO tiering, replication, legal-hold policies |
| Identity | Local users, JWT, roles | OIDC | SAML/OIDC, SCIM, organisation-wide RBAC and audit retention |
| Operations | Health endpoints, Prometheus, logs, local backup | Fleet monitoring, remote configuration and support tooling | HA, control-plane federation, compliance reports |
| Support | Community support | Commercial support and certified releases | SLA, deployment and escalation support |

The code that implements paid capabilities must be kept outside the MIT CE
distribution or loaded through well-defined interfaces. CE must compile and
run without a commercial licence or external control plane.

## Current baseline

The following capabilities already exist in `ruseon-core` and are the base for
the roadmap:

- RTSP pull for H.264/H.265 over TCP or UDP (`internal/rtsp`);
- a shared in-memory ring buffer (`internal/buffer`);
- HLS and WHEP/WebRTC live delivery (`internal/hls`, `internal/webrtc`);
- local crash-recoverable fMP4 recording and archive export
  (`internal/recorder`, `internal/archive`);
- camera, tag, folder, backup, authentication, REST and gRPC APIs
  (`internal/api`, `internal/grpc`, `pkg/auth`, `pkg/storage`);
- AI metadata ingestion and delivery to WebRTC/HLS clients
  (`internal/stream/metadata.go`);
- MQTT, signed webhooks, Prometheus metrics and structured logs.

This roadmap treats these as the maintained baseline, not as completed product
work. Each phase includes acceptance criteria that must be met before a feature
is advertised as supported.

## Phase 0 -- Production foundation

**Outcome:** a CE node can be safely exposed within a customer network and an
operator can identify a failed camera or recording path.

### CE implementation

1. **Configurable network security**
   - Move the gRPC listen address and port from `pkg/engine` into `pkg/config`.
   - Add configuration for STUN, TURN, ICE transport policy and credentials;
     remove the fixed public STUN dependency from `internal/webrtc`.
   - Add TLS for externally exposed HTTP endpoints and authenticated,
     encrypted transport for gRPC AI integrations.
   - Define supported reverse-proxy and direct-exposure deployments.

2. **Camera lifecycle and diagnostics**
   - Add explicit states: `connecting`, `online`, `degraded`, `offline`, and
     `disabled` to `internal/models` and the camera API.
   - Implement bounded reconnect with exponential backoff and jitter in
     `internal/stream`/`internal/rtsp`.
   - Report the last frame, last keyframe, source codec, current bitrate,
     reconnect count and last error per camera.
   - Emit `camera_offline`, `camera_online`, `recording_failed` and
     `storage_warning` events through the existing event bus.

3. **Access and audit baseline**
   - Replace the current all-or-nothing authenticated API with CE roles:
     `admin`, `operator`, `viewer`, and `service`.
   - Audit login, camera changes, archive viewing and archive export.
   - Keep stream tokens short-lived and add token revocation on camera/user
     permission changes.

4. **Operational contract**
   - Split liveness from readiness endpoints.
   - Extend Prometheus metrics with per-camera ingest, archive and WebRTC
     status, avoiding high-cardinality viewer labels.
   - Publish a versioned compatibility matrix for camera, codec, browser and
     operating-system combinations.

### Paid editions

- **Pro:** signed, certified release channels and optional remote diagnostics
  consent.
- **Enterprise:** extended audit retention, immutable audit export and
  organisation-wide security reporting.

### Exit criteria

- A disconnected camera reaches `offline` and produces one deduplicated event
  within the configured detection interval.
- A CE node works without public Internet access when configured with an
  on-premises TURN server or when TURN is not required.
- No anonymous endpoint exposes camera configuration, archive or AI metadata.

## Phase 1 -- Camera interoperability and operator VMS

**Outcome:** an integrator can add common IP cameras quickly; an operator can
watch, investigate and export local video without bespoke tooling.

### CE implementation

1. **ONVIF camera onboarding**
   - Add ONVIF discovery on selected network interfaces; do not scan networks
     by default.
   - Read device information, media profiles and snapshot URLs.
   - Let the user select a main stream and a substream, then validate codec,
     resolution and credentials before saving a camera.
   - Store vendor/model/firmware as inventory metadata, without making it a
     requirement for RTSP operation.

2. **Audio and PTZ**
   - Extend the RTSP pipeline to retain audio tracks and support AAC, G.711
     A-law, G.711 mu-law and Opus where the delivery protocol permits them.
   - Add ONVIF PTZ presets and directional control behind role checks.
   - Keep talkback out of this phase unless a tested camera matrix and secure
     backchannel design are available.

3. **Operator workflow**
   - Add saved 4/9/16-camera layouts, a camera detail page and current camera
     health to the embedded web UI.
   - Add an archive timeline with seek, bookmark and clip export.
   - Export a clip with camera ID, timestamps and integrity hash in a manifest.
   - Preserve the existing REST API and publish all new endpoints in Swagger.

4. **Archive safety**
   - Make recorder paths configurable rather than fixed under `recordings/`.
   - Surface retention cleanup, recovery and disk-space decisions in the UI
     and API.
   - Add tests for interrupted recording, full disk and clock discontinuity.

### Paid editions

- **Pro:** bulk camera import, CSV/API onboarding, reusable camera templates,
  vendor-specific tested profiles and priority compatibility support.
- **Enterprise:** centrally managed fleet templates, delegated site
  administration and policy-based camera onboarding.

### Exit criteria

- An integrator can discover, validate and add a supported ONVIF camera without
  manually composing an RTSP URL.
- An operator can locate and export a one-minute clip from local archive in
  less than three minutes.
- Audio support is advertised only for explicitly tested ingest-to-playback
  combinations.

## Phase 2 -- Event-driven AI VMS

**Outcome:** RUSEON provides a stable neutral layer between video sources, AI
workers and operator actions. It does not require customers to adopt one model
vendor.

### CE implementation

1. **AI connector contract**
   - Version the protobuf schema under `pkg/grpc/pb` for detections, tracks,
     zones, attributes, source timestamps and model identity.
   - Define delivery guarantees, clock rules, payload limits and behaviour for
     unknown labels.
   - Support AI workers that consume sampled frames or clips and send metadata
     back through authenticated gRPC.
   - Publish a reference connector for person/vehicle detection; it is a
     deployment example, not a mandatory inference engine.

2. **Event model and rules**
   - Add persistent event and incident models separate from raw video frames.
   - Support zones, lines, schedules, confidence thresholds, cooldowns and
     deduplication.
   - Implement rules of the form: event + camera/zone + schedule + confidence
     -> severity + notification + retention override.
   - Deliver rules through the existing MQTT and webhook paths with delivery
     history and retry visibility.

3. **Investigation UI**
   - Add an incident feed with preview frame, short clip, severity, assignee,
     acknowledgement and resolution state.
   - Overlay received bounding boxes and tracks in live and archive playback.
   - Add search by time, site, camera, zone, event type and object label.
   - Add privacy masks for video display and export; masks must be applied
     server-side for protected exports.

4. **Reference vertical pack: warehouse and industry**
   - Ship rules and labels for perimeter entry, person in restricted zone,
     vehicle movement and basic PPE events when a compatible external model is
     available.
   - Measure false-positive rate and operator acknowledgement rate per rule.

### Paid editions

- **Pro:** certified AI connectors, managed worker lifecycle, GPU worker
  profiles, advanced event packs and case-management integrations.
- **Enterprise:** central model governance, cross-site metadata search,
  organisation-level retention overrides and compliance reports.

### Exit criteria

- A compatible AI worker can submit a detection that appears in the operator
  feed, activates a rule and sends a signed webhook within two seconds at p95
  after RUSEON receives the metadata.
- Repeated detections of the same tracked object do not create alert storms.
- A user can confirm or dismiss an incident and the action is audited.

## Phase 3 -- Hybrid operations and delivery compatibility

**Outcome:** multiple autonomous edge sites can be operated centrally without
making cloud connectivity a data-plane dependency.

### CE implementation

1. **Protocol additions driven by validated demand**
   - Add an RTSP server/output for local system integration.
   - Add SRT ingest and egress for unreliable WAN links.
   - Add RTMP/WHIP ingest only when required by a confirmed customer workflow.
   - Add LL-HLS/CMAF only after a browser/device compatibility and latency plan
     is tested.

2. **Compatibility transcoding boundary**
   - Keep the default pipeline as passthrough/transmuxing.
   - Introduce a worker interface for opt-in compatibility renditions, such as
     H.265 to H.264/AAC; do not embed an always-on transcoder in the core
     stream manager.
   - Account for worker capacity and fail video delivery gracefully when an
     optional rendition is unavailable.

3. **Configuration export/import**
   - Make camera, rule, folder and policy configuration portable between CE
     nodes with signed backups and schema migrations.

### Paid editions

- **Pro:** RUSEON Control Plane for multi-site inventory, health aggregation,
  remote configuration rollout, update orchestration and commercial support.
- **Pro:** managed GPU compatibility workers and bandwidth-aware rendition
  policies.
- **Enterprise:** cross-site federation, central incident search and WAN
  delivery policies with regional deployment control.

### Exit criteria

- Loss of control-plane connectivity does not interrupt local ingest, archive,
  local rules or local playback.
- A supported SRT link recovers from temporary WAN loss without manual camera
  reconfiguration.
- Optional transcoding failure never blocks archive recording or native-codec
  viewers.

## Phase 4 -- Enterprise scale, storage and governance

**Outcome:** customers with multiple critical sites can operate RUSEON under
defined availability, retention and identity policies.

### CE implementation

- Maintain stable public REST/gRPC APIs, migration compatibility and local
  export so CE deployments are never trapped in a commercial control plane.
- Continue publishing performance and failure-mode tests for the CE data path.

### Paid editions

1. **Enterprise storage**
   - Implement the `pkg/storage` S3/MinIO backend for archive tiering.
   - Add upload queues, checksums, restore verification, lifecycle policies and
     local-cache eviction that never removes unverified remote copies.
   - Add replication and legal-hold retention policies.

2. **Enterprise identity and governance**
   - Integrate OIDC, SAML and SCIM through the Enterprise identity module.
   - Add organisation/site/camera scoped RBAC, service-account governance and
     long-term audit retention.
   - Add export approvals and privacy-aware evidence workflows.

3. **Availability and fleet operations**
   - Separate control-plane and media-node responsibilities.
   - Add node health, camera assignment, controlled failover and recovery
     procedures before claiming high availability.
   - Add documented backup/restore and disaster-recovery exercises.

### Exit criteria

- Tiered archive recovery verifies checksums before reporting a clip available.
- A node failure follows a tested recovery/failover runbook with measured RTO
  and RPO.
- Enterprise identity provisioning and access revocation are reflected in
  camera/archive access within a documented time bound.

## Quality gates for every phase

Every feature is complete only when it has all of the following:

1. Unit tests in the owning package and end-to-end coverage for the operator
   workflow where practical.
2. Race, leak and failure-mode tests for new concurrent media paths.
3. Prometheus metrics, structured logs and documented alert conditions.
4. REST/OpenAPI or protobuf documentation, configuration reference and upgrade
   notes.
5. A documented edition boundary: CE behaviour, paid integration point and
   licence failure behaviour.
6. Benchmarks based on camera-like RTSP inputs and stated hardware; synthetic
   microbenchmarks are supplementary evidence only.
7. Security review for every endpoint, secret, media URL and export path.

## Explicit non-goals for the first year

- Competing with broadcast-focused servers on every protocol, DRM, CDN or
  server-side ad insertion feature.
- Building a proprietary general-purpose AI training platform.
- Making facial recognition or public-space biometric identification a default
  capability.
- Requiring cloud storage or a commercial account for local CE recording and
  playback.

## Planning cadence

Review the roadmap quarterly against customer pilots. Promote a protocol,
connector or vertical pack only after a documented customer demand, a tested
compatibility matrix and an owner for long-term support exist. Features may
move between phases; the edition boundary and the edge-first operating model
must remain stable.
