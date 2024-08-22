// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.13;

import {Script, console2} from "forge-std/Script.sol";
import {ERC20Token} from "../src/Token.sol";

contract TokenScript is Script {
    ERC20Token public tkn;

    function setUp() public {}

    function run() public returns (address) {
        //vm.broadcast();
        vm.startBroadcast();

        tkn = new ERC20Token();
        //tkn.transfer(vm.envAddress("ETHFAUCET"), uint256(500000e18));

        vm.stopBroadcast();

        return address(tkn);
    }
}
