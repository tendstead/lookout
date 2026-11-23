import { translate, type Locale } from './i18n';

export type AudioTrack = {
  codec?: string;
  language?: string;
  channels?: number;
  bitrate_bps?: number;
};

export type SubtitleTrack = {
  codec?: string;
  language?: string;
  forced?: boolean;
  default?: boolean;
  hearing_impaired?: boolean;
};

export type TechnicalProfile = {
  source?: string;
  observed_at?: string;
  version_id?: string;
  part_id?: string;
  part_number?: number;
  part_count?: number;
  edition?: string;
  size_bytes?: number;
  duration_ms?: number;
  container?: string;
  width?: number;
  height?: number;
  bitrate_bps?: number;
  video_codec?: string;
  video_profile?: string;
  video_bitrate_bps?: number;
  frame_rate?: number;
  bit_depth?: number;
  hdr?: string;
  audio?: AudioTrack[] | null;
  subtitles?: SubtitleTrack[] | null;
};

export type FileItem = {
  adapter?: string;
  reported_path?: string;
  server_id?: string;
  server_name?: string;
  storage?: string;
  archive_member?: string;
  id: number;
  copy_revision?: string;
  relative_path: string;
  size: number;
  file_size?: number;
  season: number | null;
  episodes: number[] | null;
  numbering: string;
  observation: string;
  technical?: TechnicalProfile | null;
};

export type DifferenceGroup = {
  title_id: string;
  title_name: string;
  has_artwork?: boolean;
  season?: number | null;
  episode?: number | null;
  files: FileItem[];
};

type Value = string | number | boolean | null | undefined;

export function fileSize(file: FileItem) {
  return (
    file.file_size ??
    file.technical?.size_bytes ??
    (file.size > 0 ? file.size : undefined)
  );
}

export function reportedText(value?: string) {
  const trimmed = value?.trim();
  if (!trimmed || trimmed === '-' || trimmed.toLowerCase() === 'unknown')
    return undefined;
  return trimmed;
}

function knownValue(value: Value) {
  return typeof value === 'string'
    ? reportedText(value)?.toLowerCase()
    : (value ?? undefined);
}

const emptyLanguages = new Set([
  '',
  'und',
  'unknown',
  'undetermined',
  'undefined',
  '-',
  'zxx',
  'mis',
  'n/a',
]);
const englishLanguages = new Intl.DisplayNames('en', {
  type: 'language',
  fallback: 'none',
});
const namedLanguages = new Map<string, string>([['deutsch', 'de']]);
for (const first of 'abcdefghijklmnopqrstuvwxyz') {
  for (const second of 'abcdefghijklmnopqrstuvwxyz') {
    const tag = first + second;
    const name = englishLanguages.of(tag);
    if (name) namedLanguages.set(name.toLowerCase(), tag);
  }
}

function normalizedLanguage(reported: string | undefined) {
  const raw = reported?.trim() ?? '';
  const lower = raw.toLowerCase();
  if (emptyLanguages.has(lower)) return {};
  const input = namedLanguages.get(lower) ?? raw.replaceAll('_', '-');
  try {
    const tag = Intl.getCanonicalLocales(input)[0];
    const language = new Intl.Locale(tag).language;
    if (!language || emptyLanguages.has(language)) return {};
    if (englishLanguages.of(language)) return { key: tag, tag };
  } catch {
    // Display custom track labels as reported.
  }
  return { key: lower };
}

export function languageLabel(reported: string | undefined, locale: Locale) {
  const { key, tag } = normalizedLanguage(reported);
  if (!tag) {
    return {
      name: key ? reported!.trim() : translate(locale, 'unknownLanguage'),
    };
  }
  const { language, region } = new Intl.Locale(tag);
  const labels = new Intl.DisplayNames(locale, {
    type: 'language',
    languageDisplay: 'standard',
    fallback: 'none',
  });
  const base = labels.of(language) ?? language;
  const name = labels.of(tag) ?? (region ? `${base} (${region})` : base);
  if (
    region &&
    /^[A-Z]{2}$/.test(region) &&
    region !== 'ZZ' &&
    new Intl.DisplayNames('en', { type: 'region', fallback: 'none' }).of(region)
  ) {
    const flag = String.fromCodePoint(
      ...[...region].map((c) => c.charCodeAt(0) + 127397),
    );
    return { name, flag };
  }
  return { name };
}

type Track = AudioTrack | SubtitleTrack;
type TrackProperty = (track: Track) => Value;

const codecs: TrackProperty = (track) => track.codec;
const audioFields: TrackProperty[] = [
  codecs,
  (track) => (track as AudioTrack).channels,
  (track) => (track as AudioTrack).bitrate_bps,
];
const subtitleFields: TrackProperty[] = [
  codecs,
  (track) => (track as SubtitleTrack).forced,
  (track) => (track as SubtitleTrack).default,
  (track) => (track as SubtitleTrack).hearing_impaired,
];

function differingSets<T>(left: Set<T>, right: Set<T>) {
  return (
    left.size !== right.size || [...left].some((value) => !right.has(value))
  );
}

function tracksByLanguage(tracks: Track[]) {
  const languages = new Map<string | undefined, Track[]>();
  for (const track of tracks) {
    const { key } = normalizedLanguage(track.language);
    const group = languages.get(key) ?? [];
    group.push(track);
    languages.set(key, group);
  }
  return languages;
}

function tracksDiffer(
  left: Track[] | null | undefined,
  right: Track[] | null | undefined,
  fields: TrackProperty[],
) {
  if (!left || !right) return false;
  const a = tracksByLanguage(left);
  const b = tracksByLanguage(right);
  if (
    !a.has(undefined) &&
    !b.has(undefined) &&
    differingSets(new Set(a.keys()), new Set(b.keys()))
  ) {
    return true;
  }
  for (const [language, tracks] of a) {
    const other = b.get(language);
    if (!other || (!language && (tracks.length !== 1 || other.length !== 1)))
      continue;
    const knownFields = fields.filter((field) =>
      [...tracks, ...other].every(
        (track) => knownValue(field(track)) !== undefined,
      ),
    );
    if (!knownFields.length) continue;
    const trackTuple = (track: Track) =>
      JSON.stringify(knownFields.map((field) => knownValue(field(track))));
    if (
      differingSets(
        new Set(tracks.map(trackTuple)),
        new Set(other.map(trackTuple)),
      )
    )
      return true;
  }
  return false;
}

export function comparableCopies(
  a: FileItem,
  b: FileItem,
  crossServerOnly: boolean,
) {
  return (
    a.id !== b.id &&
    (!crossServerOnly ||
      Boolean(a.server_id && b.server_id && a.server_id !== b.server_id))
  );
}

export const scalarFields = {
  resolution: [
    (file: FileItem) => file.technical?.width,
    (file: FileItem) => file.technical?.height,
  ],
  videoCodec: [(file: FileItem) => file.technical?.video_codec],
  videoBitrate: [(file: FileItem) => file.technical?.video_bitrate_bps],
  fileSize: [fileSize],
  hdr: [(file: FileItem) => file.technical?.hdr],
  edition: [(file: FileItem) => file.technical?.edition],
  videoProfile: [(file: FileItem) => file.technical?.video_profile],
  container: [(file: FileItem) => file.technical?.container],
  fileDuration: [(file: FileItem) => file.technical?.duration_ms],
  overallBitrate: [(file: FileItem) => file.technical?.bitrate_bps],
  frameRate: [(file: FileItem) => file.technical?.frame_rate],
  bitDepth: [(file: FileItem) => file.technical?.bit_depth],
} satisfies Record<string, ((file: FileItem) => Value)[]>;

export type ComparisonField =
  keyof typeof scalarFields | 'audioTracks' | 'subtitleTracks';

export function compareCopies(files: FileItem[], crossServerOnly = true) {
  const differences = new Map<number, Set<ComparisonField>>();
  const mark = (id: number, field: ComparisonField) => {
    const current = differences.get(id) ?? new Set();
    current.add(field);
    differences.set(id, current);
  };
  for (const [index, left] of files.entries()) {
    for (const right of files.slice(index + 1)) {
      if (!comparableCopies(left, right, crossServerOnly)) continue;
      const markBoth = (field: ComparisonField) => {
        mark(left.id, field);
        mark(right.id, field);
      };
      for (const field of Object.keys(
        scalarFields,
      ) as (keyof typeof scalarFields)[]) {
        for (const read of scalarFields[field]) {
          const a = knownValue(read(left));
          const b = knownValue(read(right));
          if (a !== undefined && b !== undefined && a !== b) {
            markBoth(field);
            break;
          }
        }
      }
      if (
        tracksDiffer(left.technical?.audio, right.technical?.audio, audioFields)
      )
        markBoth('audioTracks');
      if (
        tracksDiffer(
          left.technical?.subtitles,
          right.technical?.subtitles,
          subtitleFields,
        )
      )
        markBoth('subtitleTracks');
    }
  }
  return differences;
}

export function copyPreferenceKey(file: FileItem) {
  if (!file.copy_revision) return undefined;
  return JSON.stringify([file.id, file.copy_revision]);
}

export function confirmedNumbering(file: FileItem) {
  return (
    (file.numbering === 'season_episode' || file.numbering === 'nfo') &&
    file.season != null &&
    file.season >= 0
  );
}

export function confirmedEpisodes(files: FileItem[]) {
  const seasons = new Map<number, Set<number>>();
  for (const file of files) {
    if (!confirmedNumbering(file)) continue;
    const season = file.season!;
    const episodes = seasons.get(season) ?? new Set<number>();
    for (const episode of file.episodes ?? []) {
      if (episode > 0) episodes.add(episode);
    }
    if (episodes.size) seasons.set(season, episodes);
  }
  return [...seasons]
    .sort(([a], [b]) => a - b)
    .map(([season, episodes]) => ({
      season,
      episodes: [...episodes].sort((a, b) => a - b),
    }));
}

export function episodeCode(season: number, episodes: number[]) {
  const code = (n: number) => String(n).padStart(2, '0');
  const spans: string[] = [];
  const sorted = [...new Set(episodes)].sort((a, b) => a - b);
  for (let first = 0; first < sorted.length;) {
    let last = first;
    while (sorted[last + 1] === sorted[last] + 1) last++;
    spans.push(
      last === first
        ? `E${code(sorted[first])}`
        : `E${code(sorted[first])}–E${code(sorted[last])}`,
    );
    first = last + 1;
  }
  return `S${code(season)}${spans.join(', ')}`;
}
