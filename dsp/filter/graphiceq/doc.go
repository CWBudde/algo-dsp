// Package graphiceq designs octave graphic equalisers as a hybrid of IIR
// sections and one linear-phase FIR.
//
// This is a structure-specific companion to [github.com/cwbudde/algo-dsp/dsp/filter/mixedphase],
// not a general mixed-phase design. It only applies when the target is a set of
// band gains, but for that target it buys latency that no general FIR method
// can: the lowest bands move into recursive sections that have no latency
// budget at all, and the remaining FIR only has to resolve the bands above the
// split.
//
// The construction follows Bruschi, Välimäki, Liski and Cecchi, “Digital Filter
// Design for Low-Latency Graphic Equalization” (DAFx 2022), which replaces the
// lowest linear-phase band with a shelving filter and reports roughly half the
// latency of the all-FIR design.
//
// # Why the latency halves
//
// A linear-phase FIR must be long enough to resolve the narrowest feature it
// shapes. Octave band centres double, so the lowest band the FIR still handles
// sets the length, and moving one band into the IIR part doubles that centre
// frequency and therefore halves the required tap count. [DefaultLength]
// implements exactly this rule, so each band moved across the split halves the
// reported latency.
//
// # What it costs
//
// The IIR part is a cascade of low shelves, and the FIR only corrects it where
// the FIR is long enough to act — which is above the split. Whatever the
// cascade gets wrong at the offloaded bands therefore stays in the response,
// and that is the price of the latency.
//
// On ten octave bands from 31.25 Hz at 48 kHz with the gains
// 6, -3, 0, 4, -6, 2, 0, -2, 5, 0 dB:
//
//	IIR bands  taps  latency  RMS dB  peak dB
//	        0  3073     1536   0.014    0.580
//	        1  1537      768   0.016    0.603
//	        2   769      384   0.024    0.478
//	        3   385      192   0.048    0.774
//	        4   193       96   0.068    0.914
//
// The first row is the all-FIR reference. Offloading one band therefore halves
// the latency at a peak error that is unchanged within measurement noise, which
// is the DAFx result; offloading four halves it four times over while the peak
// error grows by less than a factor of two.
//
// Against an all-FIR design cut to the same tap count — the comparison that
// decides whether the split is worth anything — the hybrid wins throughout:
//
//	latency  hybrid RMS/peak dB  all-FIR RMS/peak dB
//	    768        0.016/0.603          0.049/1.117
//	    384        0.024/0.478          0.113/2.665
//	    192        0.048/0.774          0.242/4.472
//	     96        0.068/0.914          0.297/4.626
//
// # When it fails
//
// The shelves step monotonically from one band gain to the next, so a target
// that reverses direction at every octave is beyond them. With gains
// alternating between +12 and -12 dB the peak error is 3.6 dB for the all-FIR
// design, 4.0 dB with one band offloaded and 19.6 dB with two. The split is
// only worth taking when the offloaded region is smooth on the scale of the
// band spacing, which is the normal case for a room or loudspeaker correction
// and not the case for an EQ used as a comb.
//
// All of these numbers come from the package tests and examples/graphiceq, so
// they can be re-derived rather than trusted.
//
// # Scope
//
// The design is deliberately one pass: shelf gains are taken directly from the
// differences between requested band gains rather than solved for. A
// least-squares interaction solve in the manner of Välimäki and Liski would
// reduce the cascade error, and would be the next step if the offloaded part
// grows beyond the few bands measured above.
package graphiceq
