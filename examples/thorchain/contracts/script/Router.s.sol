// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.13;

import {Script, console2} from "forge-std/Script.sol";
import {THORChain_Router} from "../src/THORChain_Router.sol";

contract RouterScript is Script {
    THORChain_Router public router;

    function setUp() public {}

    function run() public returns (address) {
        vm.broadcast();
        router = new THORChain_Router(address(0x3155BA85D5F96b2d030a4966AF206230e46849cb));
        return address(router);
    }
}
