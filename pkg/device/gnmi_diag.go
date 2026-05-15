package device

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	gnoisystem "github.com/openconfig/gnoi/system"
	gnoitypes "github.com/openconfig/gnoi/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// dialGNOI opens a gRPC connection suitable for gNOI service calls.
// Arista co-serves gNOI on the same address/port as gNMI, so dial
// parameters mirror the gNMI configuration; we keep this client
// separate because gnmic's target does not expose its grpc.ClientConn.
func (d *GNMIDevice) dialGNOI(ctx context.Context) (*grpc.ClientConn, error) {
	addr := net.JoinHostPort(d.Config.Host, strconv.Itoa(d.Config.Port))
	var creds credentials.TransportCredentials
	switch {
	case d.Config.Plaintext:
		creds = insecure.NewCredentials()
	case d.Config.Insecure:
		creds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	default:
		creds = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
}

// gnoiAuthCtx attaches per-RPC credentials in the gNOI metadata form
// Arista expects: username + password headers on the outgoing context.
func (d *GNMIDevice) gnoiAuthCtx(ctx context.Context) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{
		"username": d.Config.Username,
		"password": d.Config.Password,
	}))
}

func (d *GNMIDevice) Ping(ctx context.Context, opts PingOpts) (*PingResult, error) {
	if opts.Destination == "" {
		return nil, fmt.Errorf("Ping: destination is required")
	}

	conn, err := d.dialGNOI(ctx)
	if err != nil {
		return nil, fmt.Errorf("gNOI dial %s: %w", d.Config.Name, err)
	}
	defer conn.Close()

	req := &gnoisystem.PingRequest{
		Destination:     opts.Destination,
		Source:          opts.Source,
		Count:           int32(opts.Count),
		Size:            int32(opts.Size),
		Interval:        int64(opts.Interval),
		Wait:            int64(opts.Wait),
		DoNotFragment:   opts.DoNotFragment,
		DoNotResolve:    opts.DoNotResolve,
		L3Protocol:      l3ToProto(opts.L3Protocol),
		NetworkInstance: opts.VRF,
	}

	stream, err := gnoisystem.NewSystemClient(conn).Ping(d.gnoiAuthCtx(ctx), req)
	if err != nil {
		return nil, fmt.Errorf("gNOI Ping RPC: %w", err)
	}

	result := &PingResult{Destination: opts.Destination}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gNOI Ping stream: %w", err)
		}
		// The summary message carries Sent > 0 (and Sequence == 0); the
		// per-echo messages carry Sequence > 0. We dispatch on Sent so
		// that a future device that omits Sequence on the per-echo
		// path still produces a valid Echoes slice.
		if resp.GetSent() > 0 {
			result.Stats = PingStats{
				Sent:     int(resp.GetSent()),
				Received: int(resp.GetReceived()),
				MinRTT:   time.Duration(resp.GetMinTime()),
				AvgRTT:   time.Duration(resp.GetAvgTime()),
				MaxRTT:   time.Duration(resp.GetMaxTime()),
				StdDev:   time.Duration(resp.GetStdDev()),
				Duration: time.Duration(resp.GetTime()),
			}
			if resp.GetSent() > 0 {
				result.Stats.Loss = 1 - float64(resp.GetReceived())/float64(resp.GetSent())
			}
			if result.Source == "" {
				result.Source = resp.GetSource()
			}
		} else {
			result.Echoes = append(result.Echoes, PingEcho{
				Sequence: int(resp.GetSequence()),
				Bytes:    int(resp.GetBytes()),
				TTL:      int(resp.GetTtl()),
				RTT:      time.Duration(resp.GetTime()),
			})
			if result.Source == "" {
				result.Source = resp.GetSource()
			}
		}
	}
	return result, nil
}

func (d *GNMIDevice) Traceroute(ctx context.Context, opts TracerouteOpts) (*TracerouteResult, error) {
	if opts.Destination == "" {
		return nil, fmt.Errorf("Traceroute: destination is required")
	}

	conn, err := d.dialGNOI(ctx)
	if err != nil {
		return nil, fmt.Errorf("gNOI dial %s: %w", d.Config.Name, err)
	}
	defer conn.Close()

	req := &gnoisystem.TracerouteRequest{
		Destination:     opts.Destination,
		MaxTtl:          int32(opts.MaxTTL),
		Wait:            int64(opts.Wait),
	}
	if opts.Source != "" {
		req.Source = opts.Source
	}
	if opts.VRF != "" {
		req.NetworkInstance = opts.VRF
	}
	if opts.InitialTTL > 0 {
		req.InitialTtl = uint32(opts.InitialTTL)
	}
	if opts.DoNotFragment {
		req.DoNotFragment = true
	}
	if opts.DoNotResolve {
		req.DoNotResolve = true
	}
	if opts.L3Protocol != L3ProtocolAuto {
		req.L3Protocol = l3ToProto(opts.L3Protocol)
	}

	stream, err := gnoisystem.NewSystemClient(conn).Traceroute(d.gnoiAuthCtx(ctx), req)
	if err != nil {
		return nil, fmt.Errorf("gNOI Traceroute RPC: %w", err)
	}

	result := &TracerouteResult{Destination: opts.Destination}
	hopIndex := map[int32]int{}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gNOI Traceroute stream: %w", err)
		}
		// Header message: destination_address/name are set on the first
		// response, no hop number. Per-probe messages have hop > 0.
		if resp.GetHop() == 0 {
			if a := resp.GetDestinationAddress(); a != "" {
				result.Destination = a
			}
			if n := resp.GetDestinationName(); n != "" {
				result.DestinationName = n
			}
			if s := resp.GetPacketSize(); s > 0 {
				result.PacketSize = int(s)
			}
			continue
		}
		hopNum := resp.GetHop()
		idx, ok := hopIndex[hopNum]
		if !ok {
			idx = len(result.Hops)
			hopIndex[hopNum] = idx
			result.Hops = append(result.Hops, TracerouteHop{Number: int(hopNum)})
		}
		result.Hops[idx].Probes = append(result.Hops[idx].Probes, TracerouteProbe{
			Address: resp.GetAddress(),
			Name:    resp.GetName(),
			RTT:     time.Duration(resp.GetRtt()),
			State:   tracerouteStateFromProto(resp.GetState()),
		})
	}
	return result, nil
}

func l3ToProto(l L3Protocol) gnoitypes.L3Protocol {
	switch l {
	case L3ProtocolIPv4:
		return gnoitypes.L3Protocol_IPV4
	case L3ProtocolIPv6:
		return gnoitypes.L3Protocol_IPV6
	default:
		return gnoitypes.L3Protocol_UNSPECIFIED
	}
}

func tracerouteStateFromProto(s gnoisystem.TracerouteResponse_State) TracerouteState {
	switch s {
	case gnoisystem.TracerouteResponse_DEFAULT, gnoisystem.TracerouteResponse_NONE:
		return TracerouteStateNone
	case gnoisystem.TracerouteResponse_ICMP:
		return TracerouteStateICMP
	case gnoisystem.TracerouteResponse_HOST_UNREACHABLE:
		return TracerouteStateHostUnreachable
	case gnoisystem.TracerouteResponse_NETWORK_UNREACHABLE:
		return TracerouteStateNetworkUnreachable
	case gnoisystem.TracerouteResponse_PROTOCOL_UNREACHABLE:
		return TracerouteStateProtocolUnreachable
	case gnoisystem.TracerouteResponse_SOURCE_ROUTE_FAILED:
		return TracerouteStateSourceRouteFailed
	case gnoisystem.TracerouteResponse_FRAGMENTATION_NEEDED:
		return TracerouteStateFragmentationNeeded
	case gnoisystem.TracerouteResponse_PROHIBITED:
		return TracerouteStateProhibited
	case gnoisystem.TracerouteResponse_PRECEDENCE_VIOLATION:
		return TracerouteStatePrecedenceViolation
	case gnoisystem.TracerouteResponse_PRECEDENCE_CUTOFF:
		return TracerouteStatePrecedenceCutoff
	default:
		return TracerouteStateUnknown
	}
}
