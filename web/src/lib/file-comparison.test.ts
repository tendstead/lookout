import { describe, expect, it } from 'vitest';
import {
  compareCopies,
  confirmedEpisodes,
  copyPreferenceKey,
  languageLabel,
  type FileItem,
} from './file-comparison';

function file(input: Partial<FileItem> = {}): FileItem {
  return {
    id: 1,
    copy_revision: 'first',
    server_id: 'one',
    relative_path: 'Demo.S01E02.mkv',
    size: 0,
    season: 1,
    episodes: [2],
    numbering: 'season_episode',
    observation: 'observed',
    ...input,
  };
}

describe('file comparisons', () => {
  it('compares known values across servers and keeps known zero sizes', () => {
    const left = file({
      file_size: 0,
      technical: { width: 1920, height: 1080, video_bitrate_bps: 1000 },
    });
    const right = file({
      id: 2,
      server_id: 'two',
      file_size: 10,
      technical: { width: 3840, height: 2160 },
    });
    expect([...compareCopies([left, right]).get(1)!].sort()).toEqual([
      'fileSize',
      'resolution',
    ]);
    const sameServer = [left, { ...right, server_id: 'one' }];
    expect(compareCopies(sameServer).size).toBe(0);
    expect(compareCopies(sameServer, false).size).toBe(2);
  });

  it('compares editions and treats provider placeholders as missing', () => {
    const left = file({
      technical: { video_codec: 'Unknown', edition: 'Theatrical' },
    });
    const right = file({
      id: 2,
      server_id: 'two',
      technical: { video_codec: 'h264', edition: 'Extended' },
    });
    expect([...compareCopies([left, right]).get(1)!]).toEqual(['edition']);
  });

  it('compares known track properties independent of order and aliases', () => {
    const left = file({
      technical: {
        audio: [
          { language: 'de', codec: 'AAC' },
          { language: 'eng', bitrate_bps: 1000 },
        ],
        subtitles: [{ language: 'eng', forced: false }],
      },
    });
    const right = file({
      id: 2,
      server_id: 'two',
      technical: {
        audio: [{ language: 'English' }, { language: 'German', codec: 'aac' }],
        subtitles: [{ language: 'en', forced: true }],
      },
    });
    expect([...compareCopies([left, right]).get(1)!]).toEqual([
      'subtitleTracks',
    ]);
  });

  it('keeps codec and channel counts associated within each track', () => {
    const left = file({
      technical: {
        audio: [
          { language: 'en', codec: 'AAC', channels: 2 },
          { language: 'en', codec: 'AC3', channels: 6 },
        ],
      },
    });
    const right = file({
      id: 2,
      server_id: 'two',
      technical: {
        audio: [
          { language: 'en', codec: 'AC3', channels: 2 },
          { language: 'en', codec: 'AAC', channels: 6 },
        ],
      },
    });
    expect(compareCopies([left, right]).get(1)?.has('audioTracks')).toBe(true);
  });

  it('ignores undetermined language but compares explicitly empty tracks', () => {
    const left = file({ technical: { audio: [] } });
    const right = file({
      id: 2,
      server_id: 'two',
      technical: { audio: [{ language: 'und' }] },
    });
    expect(compareCopies([left, right]).size).toBe(0);
    expect([
      ...compareCopies([
        left,
        { ...right, technical: { audio: [{ language: 'deu' }] } },
      ]).get(1)!,
    ]).toEqual(['audioTracks']);
  });

  it('includes every confirmed episode and excludes uncertain numbering', () => {
    expect(
      confirmedEpisodes([
        file({ episodes: [3, 2, 2] }),
        file({ id: 2, episodes: [8], numbering: 'unknown' }),
        file({ id: 3, season: 0, episodes: [1], numbering: 'nfo' }),
      ]),
    ).toEqual([
      { season: 0, episodes: [1] },
      { season: 1, episodes: [2, 3] },
    ]);
  });

  it('changes the dismissal key after an observed version change', () => {
    expect(copyPreferenceKey(file())).toBe(
      copyPreferenceKey(file({ server_id: 'renamed' })),
    );
    expect(copyPreferenceKey(file())).not.toBe(
      copyPreferenceKey(file({ copy_revision: 'second' })),
    );
    expect(
      copyPreferenceKey(file({ copy_revision: undefined })),
    ).toBeUndefined();
  });
});

describe('track language labels', () => {
  it('renders only a supported explicit region as a flag', () => {
    expect(languageLabel('pt-BR', 'en')).toEqual({
      name: 'Portuguese (Brazil)',
      flag: '🇧🇷',
    });
    expect(languageLabel('por', 'en')).toEqual({ name: 'Portuguese' });
    expect(languageLabel('pt-ZZ', 'en')).toEqual({
      name: 'Portuguese (Unknown Region)',
    });
    expect(languageLabel('pt-QQ', 'en')).toEqual({ name: 'Portuguese (QQ)' });
    expect(languageLabel('und', 'en')).toEqual({ name: 'Unknown language' });
    expect(languageLabel('undefined', 'de')).toEqual({
      name: 'Unbekannte Sprache',
    });
    expect(languageLabel('Deutsch', 'en')).toEqual({ name: 'German' });
    expect(languageLabel('custom audio', 'en')).toEqual({
      name: 'custom audio',
    });
  });
});
