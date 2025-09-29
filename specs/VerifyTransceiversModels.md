# Test Specification: VerifyTransceiversModels

## Purpose
Verify that each interface has the **exact transceiver model** specified in the test criteria - no more, no less.

## Test Definition Structure

```yaml
anta.tests.hardware:
  - VerifyTransceiversModels:
      interfaces:
        - name: Ethernet1
          model: "SFP-10G-SR"
        - name: Ethernet2
          model: "SFP-10G-SR"
        - name: Ethernet49/1
          model: "QSFP-100G-LR4"
        - name: Ethernet50/1
          model: "QSFP-40G-SR4"
```

## Test Logic

```
For each interface in the test criteria:
  1. Get the transceiver information from the interface
  2. Extract the model identifier from the transceiver data
  3. Compare against the expected model (exact string match)
  4. Pass if match, fail if different or missing
```

## Implementation Approach

The test would:
1. Execute: `show inventory` or `show interfaces transceiver`
2. For each interface, extract the model field
3. Perform exact string comparison
4. Report any mismatches

## Example with Real-World Models

```yaml
anta.tests.hardware:
  - VerifyTransceiversModels:
      interfaces:
        # 10G Short-range optics for rack connections
        - name: Ethernet1
          model: "SFP-10G-SR"
        - name: Ethernet2
          model: "SFP-10G-SR"

        # 100G Long-range for spine uplinks
        - name: Ethernet49/1
          model: "QSFP-100G-LR4"
        - name: Ethernet50/1
          model: "QSFP-100G-LR4"

        # 1G copper for management
        - name: Ethernet48
          model: "SFP-1G-T"
```

## Expected Output

### Success Case:
```
✓ All transceivers match expected models
```

### Failure Cases:
```
✗ Interface: Ethernet1
  Expected model: SFP-10G-SR
  Actual model: SFP-10G-LR

✗ Interface: Ethernet49/1
  Expected model: QSFP-100G-LR4
  Actual model: Not Present (empty slot)

✗ Interface: Ethernet50/1
  Expected model: QSFP-100G-LR4
  Actual model: QSFP-100G-SR4
```

## Key Points for Clarity

1. **Model** = The exact product identifier/SKU from the transceiver's EEPROM
2. **Exact Match** = String must match completely (case-sensitive or normalized)
3. **No Partial Matches** = "SFP-10G-SR" ≠ "SFP-10G-SR-A"
4. **Empty Slots** = Reported as failure with "Not Present" message

This specification is clear because it:
- Has a single responsibility: verify model matches exactly
- Uses the simplest possible data structure
- Makes the pass/fail criteria unambiguous
- Provides clear error messages for troubleshooting