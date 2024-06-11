#include "FLAC/stream_decoder.h"
#include "FLAC/stream_encoder.h"

#include "_cgo_export.h"

extern int
get_decoder_channels(FLAC__StreamMetadata *metadata)
{
     return metadata->data.stream_info.channels;
}

extern int
get_decoder_depth(FLAC__StreamMetadata *metadata)
{
     return metadata->data.stream_info.bits_per_sample;
}

extern int
get_decoder_rate(FLAC__StreamMetadata *metadata)
{
     return metadata->data.stream_info.sample_rate;
}

extern FLAC__uint64
get_total_samples(FLAC__StreamMetadata *metadata)
{
    return metadata->data.stream_info.total_samples;
}

void
decoderErrorCallback_cgo(const FLAC__StreamDecoder *decoder,
                 FLAC__StreamDecoderErrorStatus status,
                 void *data)
{
    decoderErrorCallback((FLAC__StreamDecoder *)decoder, status, data);
}

void
decoderMetadataCallback_cgo(const FLAC__StreamDecoder *decoder,
                const FLAC__StreamMetadata *metadata,
                void *data)
{
    decoderMetadataCallback((FLAC__StreamDecoder *)decoder,
                (FLAC__StreamMetadata *)metadata, data);
}

FLAC__StreamDecoderWriteStatus
decoderWriteCallback_cgo(const FLAC__StreamDecoder *decoder,
                 const FLAC__Frame *frame,
                 const FLAC__int32 **buffer,
                 void *data)
{
    return decoderWriteCallback((FLAC__StreamDecoder *)decoder,
                (FLAC__Frame *)frame,
                (FLAC__int32 **)buffer, data);
}
