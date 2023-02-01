// Copyright 2021-2022, Offchain Labs, Inc.
// For license information, see https://github.com/nitro/blob/master/LICENSE
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import "../rollup/AbsBridgeCreator.sol";
import "../bridge/ERC20Bridge.sol";
import "../bridge/IERC20Bridge.sol";
import "../bridge/ERC20Inbox.sol";

contract ERC20BridgeCreator is AbsBridgeCreator {
    constructor() AbsBridgeCreator() {
        bridgeTemplate = new ERC20Bridge();
        inboxTemplate = new ERC20Inbox();
    }

    function createBridge(
        address adminProxy,
        address rollup,
        address nativeToken,
        ISequencerInbox.MaxTimeVariation memory maxTimeVariation
    )
        external
        returns (
            IBridge,
            SequencerInbox,
            IInbox,
            RollupEventInbox,
            Outbox
        )
    {
        return _createBridge(adminProxy, rollup, nativeToken, maxTimeVariation);
    }

    function _initializeBridge(
        IBridge bridge,
        IOwnable rollup,
        address nativeToken
    ) internal override {
        IERC20Bridge(address(bridge)).initialize(IOwnable(rollup), nativeToken);
    }
}
