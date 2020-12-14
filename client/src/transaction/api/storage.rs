//! Client for service defined in go/storage/api.
use grpcio::{CallOption, Channel, Client, Result};
use serde::{Deserialize, Serialize};
use serde_cbor::value::Value;

use oasis_core_runtime::{
    common::{crypto::hash::Hash, namespace::Namespace},
    storage::mkvs::{sync, WriteLog},
};

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ApplyRequest {
    pub namespace: Namespace,
    pub src_round: u64,
    pub src_root: Hash,
    pub dst_round: u64,
    pub dst_root: Hash,
    pub writelog: WriteLog,
}

// XXX: This should be an actual receipt once somebody needs it from Rust.
/// A storage receipt.
type Receipt = Value;

grpc_method!(
    METHOD_APPLY,
    "/oasis-core.Storage/Apply",
    ApplyRequest,
    Receipt
);
grpc_method!(
    METHOD_SYNC_GET,
    "/oasis-core.Storage/SyncGet",
    sync::GetRequest,
    sync::ProofResponse
);
grpc_method!(
    METHOD_SYNC_GET_PREFIXES,
    "/oasis-core.Storage/SyncGetPrefixes",
    sync::GetPrefixesRequest,
    sync::ProofResponse
);
grpc_method!(
    METHOD_SYNC_ITERATE,
    "/oasis-core.Storage/SyncIterate",
    sync::IterateRequest,
    sync::ProofResponse
);

/// A (simplified) storage gRPC service client.
#[derive(Clone)]
pub struct StorageClient {
    client: Client,
}

impl StorageClient {
    /// Create a new storage client.
    pub fn new(channel: Channel) -> Self {
        StorageClient {
            client: Client::new(channel),
        }
    }

    // Apply applies the write log against the storage.
    pub fn apply(&self, request: &ApplyRequest, opt: CallOption) -> Result<Receipt> {
        self.client.unary_call(&METHOD_APPLY, &request, opt)
    }

    /// Fetch a single key and return the corresponding proof.
    pub fn sync_get(
        &self,
        request: &sync::GetRequest,
        opt: CallOption,
    ) -> Result<sync::ProofResponse> {
        self.client.unary_call(&METHOD_SYNC_GET, &request, opt)
    }

    /// Fetch all keys under the given prefixes and return the corresponding proofs.
    pub fn sync_get_prefixes(
        &self,
        request: &sync::GetPrefixesRequest,
        opt: CallOption,
    ) -> Result<sync::ProofResponse> {
        self.client
            .unary_call(&METHOD_SYNC_GET_PREFIXES, &request, opt)
    }

    /// Seek to a given key and then fetch the specified number of following items
    /// based on key iteration order.
    pub fn sync_iterate(
        &self,
        request: &sync::IterateRequest,
        opt: CallOption,
    ) -> Result<sync::ProofResponse> {
        self.client.unary_call(&METHOD_SYNC_ITERATE, &request, opt)
    }
}
