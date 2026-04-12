# References

Academic papers, textbooks, and resources underlying netlens.

## Network Tomography — Foundational

1. **Vardi (1996)** — Y. Vardi, "Network Tomography: Estimating Source-Destination Traffic Intensities from Link Data," *Journal of the American Statistical Association*, vol. 91, no. 433, pp. 365–377, 1996. DOI: [10.1080/01621459.1996.10476697](https://doi.org/10.1080/01621459.1996.10476697)
   Introduced the term "network tomography" and applied the EM algorithm to estimate OD traffic matrices from link counts.

2. **Tebaldi & West (1998)** — C. Tebaldi and M. West, "Bayesian Inference on Network Traffic Using Link Count Data," *Journal of the American Statistical Association*, vol. 93, no. 442, pp. 557–573, 1998. DOI: [10.1080/01621459.1998.10473707](https://doi.org/10.1080/01621459.1998.10473707)
   Bayesian hierarchical model with MCMC sampling for OD flow estimation.

3. **Cao, Davis, Vander Wiel, Yu (2000)** — J. Cao, D. Davis, S. Vander Wiel, and B. Yu, "Time-Varying Network Tomography: Router Link Data," *Journal of the American Statistical Association*, vol. 95, no. 452, pp. 1063–1075, 2000. DOI: [10.1080/01621459.2000.10474303](https://doi.org/10.1080/01621459.2000.10474303)
   Scalable method-of-moments estimator exploiting second-order statistics of link counts.

4. **Coates, Hero, Nowak, Yu (2002)** — M. Coates, A. Hero, R. Nowak, and B. Yu, "Internet Tomography," *IEEE Signal Processing Magazine*, vol. 19, no. 3, pp. 47–65, 2002. DOI: [10.1109/79.998081](https://doi.org/10.1109/79.998081)
   Tutorial on applying statistical signal processing to infer internal network characteristics from edge measurements.

5. **Castro, Coates, Liang, Nowak, Yu (2004)** — R. Castro, M. Coates, G. Liang, R. Nowak, and B. Yu, "Network Tomography: Recent Developments," *Statistical Science*, vol. 19, no. 3, pp. 499–517, 2004. DOI: [10.1214/088342304000000422](https://doi.org/10.1214/088342304000000422)
   Comprehensive survey covering link-level, OD-flow, and topology inference problems.

6. **Zhang, Roughan, Duffield, Greenberg (2003)** — Y. Zhang, M. Roughan, N. Duffield, and A. Greenberg, "Fast Accurate Computation of Large-Scale IP Traffic Matrices from Link Loads," *ACM SIGMETRICS*, 2003. DOI: [10.1145/781027.781053](https://doi.org/10.1145/781027.781053)
   Combined gravity-model priors with tomographic constraints (tomogravity) for scalable OD matrix estimation.

7. **He, Ma, Swami, Towsley (2021)** — T. He, L. Ma, A. Swami, and D. Towsley, *Network Tomography: Identifiability, Measurement Design, and Network State Inference*, Cambridge University Press, 2021. ISBN: 978-1-108-42105-4.
   First dedicated textbook on network tomography; covers identifiability theory, optimal monitor placement, and Boolean/additive tomography.

## Identifiability & Measurement Design

8. **Chen, Cao, Bu (2007)** — A. Chen, J. Cao, and T. Bu, "Network Tomography: Identifiability and Fourier Domain Estimation," *IEEE INFOCOM*, pp. 1875–1883, 2007. DOI: [10.1109/INFCOM.2007.219](https://doi.org/10.1109/INFCOM.2007.219)
   Necessary and sufficient conditions on the routing matrix for unique identifiability of link-level parameters.

9. **Zhao, Chen, Bindel (2009)** — Y. Zhao, Y. Chen, and D. Bindel, "Towards Unbiased End-to-End Network Diagnosis," *IEEE/ACM Transactions on Networking*, vol. 17, no. 6, pp. 1724–1737, 2009. DOI: [10.1109/TNET.2009.2022158](https://doi.org/10.1109/TNET.2009.2022158)
   Measurement design and optimal probe path selection for Boolean network tomography.

## Solver Methods

10. **Tikhonov & Arsenin (1977)** — A. N. Tikhonov and V. Y. Arsenin, *Solutions of Ill-Posed Problems*, Winston & Sons, 1977. ISBN: 978-0-470-99124-4.
    Definitive reference for Tikhonov regularization of ill-posed inverse problems.

11. **Lawson & Hanson (1974)** — C. L. Lawson and R. J. Hanson, *Solving Least Squares Problems*, Prentice-Hall, 1974. Reprinted by SIAM, 1995. ISBN: 978-0-89871-356-5.
    Non-negative least squares (NNLS) algorithm and constrained least squares.

12. **Hansen (1987)** — P. C. Hansen, "The Truncated SVD as a Method for Regularization," *BIT Numerical Mathematics*, vol. 27, no. 4, pp. 534–553, 1987. DOI: [10.1007/BF01937276](https://doi.org/10.1007/BF01937276)
    Truncated SVD as regularization for discrete ill-posed problems.

13. **Golub, Heath, Wahba (1979)** — G. H. Golub, M. Heath, and G. Wahba, "Generalized Cross-Validation as a Method for Choosing a Good Ridge Parameter," *Technometrics*, vol. 21, no. 2, pp. 215–223, 1979. DOI: [10.1080/00401706.1979.10489751](https://doi.org/10.1080/00401706.1979.10489751)
    GCV for automatic selection of regularization parameters without knowledge of the noise level.

14. **Hansen (1992)** — P. C. Hansen, "Analysis of Discrete Ill-Posed Problems by Means of the L-Curve," *SIAM Review*, vol. 34, no. 4, pp. 561–580, 1992. DOI: [10.1137/1034115](https://doi.org/10.1137/1034115)
    L-curve criterion for choosing the regularization parameter.

15. **Boyd, Parikh, Chu, Peleato, Eckstein (2011)** — S. Boyd, N. Parikh, E. Chu, B. Peleato, and J. Eckstein, "Distributed Optimization and Statistical Learning via the Alternating Direction Method of Multipliers," *Foundations and Trends in Machine Learning*, vol. 3, no. 1, pp. 1–122, 2011. DOI: [10.1561/2200000016](https://doi.org/10.1561/2200000016)
    Comprehensive monograph on ADMM for convex optimization.

16. **Candes, Romberg, Tao (2006)** — E. J. Candes, J. Romberg, and T. Tao, "Robust Uncertainty Principles: Exact Signal Recovery from Highly Incomplete Frequency Information," *IEEE Transactions on Information Theory*, vol. 52, no. 2, pp. 489–509, 2006. DOI: [10.1109/TIT.2005.862083](https://doi.org/10.1109/TIT.2005.862083)
    Foundational compressed sensing paper proving sparse recovery from underdetermined systems.

17. **Donoho (2006)** — D. L. Donoho, "Compressed Sensing," *IEEE Transactions on Information Theory*, vol. 52, no. 4, pp. 1289–1306, 2006. DOI: [10.1109/TIT.2006.871582](https://doi.org/10.1109/TIT.2006.871582)
    Theoretical foundations of compressed sensing and L1 minimization.

18. **Efron (1979)** — B. Efron, "Bootstrap Methods: Another Look at the Jackknife," *The Annals of Statistics*, vol. 7, no. 1, pp. 1–26, 1979. DOI: [10.1214/aos/1176344552](https://doi.org/10.1214/aos/1176344552)
    Original paper introducing bootstrap resampling.

19. **Firooz & Roy (2010)** — M. H. Firooz and S. Roy, "Network Tomography via Compressed Sensing," *IEEE GLOBECOM*, pp. 1–5, 2010. DOI: [10.1109/GLOCOM.2010.5684240](https://doi.org/10.1109/GLOCOM.2010.5684240)
    Applied compressed sensing to infer link-level metrics from path measurements.

## Measurement Infrastructure & Topology

20. **Augustin et al. (2006)** — B. Augustin, X. Cuvellier, B. Orgogozo, F. Viger, T. Friedman, M. Latapy, C. Magnien, and R. Teixeira, "Avoiding Traceroute Anomalies with Paris Traceroute," *ACM IMC*, 2006. DOI: [10.1145/1177080.1177100](https://doi.org/10.1145/1177080.1177100)
    Per-flow traceroute that avoids ECMP-induced false links and loops.

21. **Augustin, Friedman, Teixeira (2011)** — B. Augustin, T. Friedman, and R. Teixeira, "Measuring Multipath Routing in the Internet," *IEEE/ACM Transactions on Networking*, vol. 19, no. 3, pp. 830–840, 2011. DOI: [10.1109/TNET.2010.2096232](https://doi.org/10.1109/TNET.2010.2096232)
    Quantifies ECMP prevalence and its distortion of traceroute-inferred topologies.

22. **Keys, Hyun, Luckie, claffy (2014)** — K. Keys, Y. Hyun, M. Luckie, and k claffy, "Internet-Scale IPv4 Alias Resolution with MIDAR," *IEEE/ACM Transactions on Networking*, vol. 22, no. 4, 2014. DOI: [10.1109/TNET.2013.2275735](https://doi.org/10.1109/TNET.2013.2275735)
    Monotonic ID-based alias resolution at Internet scale.

23. **Knight, Nguyen, Falkner, Bowden, Roughan (2011)** — S. Knight, H. X. Nguyen, N. Falkner, R. Bowden, and M. Roughan, "The Internet Topology Zoo," *IEEE JSAC*, vol. 29, no. 9, 2011. DOI: [10.1109/JSAC.2011.111002](https://doi.org/10.1109/JSAC.2011.111002)
    Curated dataset of 261 real ISP/research network topologies.

24. **Barabasi & Albert (1999)** — A.-L. Barabasi and R. Albert, "Emergence of Scaling in Random Networks," *Science*, vol. 286, no. 5439, pp. 509–512, 1999. DOI: [10.1126/science.286.5439.509](https://doi.org/10.1126/science.286.5439.509)
    Preferential attachment model producing scale-free graphs.

25. **Waxman (1988)** — B. M. Waxman, "Routing of Multipoint Connections," *IEEE JSAC*, vol. 6, no. 9, pp. 1617–1622, 1988. DOI: [10.1109/49.12889](https://doi.org/10.1109/49.12889)
    Geographic random graph model with distance-dependent edge probability.

26. **Katz-Bassett et al. (2006)** — E. Katz-Bassett, J. P. John, A. Krishnamurthy, D. Wetherall, T. Anderson, and Y. Chawathe, "Towards IP Geolocation Using Delay and Topology Measurements," *ACM IMC*, 2006. DOI: [10.1145/1177080.1177090](https://doi.org/10.1145/1177080.1177090)
    Speed-of-light propagation as a hard lower bound on RTT for geolocating routers.

## Mapping to netlens Components

| netlens component | Key references |
|-------------------|---------------|
| Tikhonov solver | [10], [13], [14] |
| NNLS solver | [11] |
| TSVD solver | [12] |
| ADMM solver | [15], [16], [17], [19] |
| Vardi EM solver | [1] |
| Tomogravity solver | [6] |
| Bootstrap CI | [18] |
| Identifiability analysis | [5], [7], [8] |
| Measurement design (`plan`) | [7], [9] |
| Topology Zoo loader | [23] |
| Synthetic generators | [24], [25] |
| Paris traceroute / ECMP | [20], [21] |
| IP alias resolution | [22] |
| Speed-of-light validation | [26] |
